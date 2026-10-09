package docker

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// NormalizeGitURL normalizes a Git repository URL for reliable comparison
func NormalizeGitURL(raw string) string {
	u := strings.TrimSpace(raw)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

// ValidateAndFindTargets finds matching target containers and validates the provided secret token
func ValidateAndFindTargets(ctx context.Context, action, imageURL, gitURL, containerRegex, token string) ([]ResolvedTarget, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var targets []ResolvedTarget

	var re *regexp.Regexp
	if containerRegex != "" {
		re, err = regexp.Compile(containerRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid container regex: %v", err)
		}
	}

	for _, c := range containers {
		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		isAutoDeploy := false
		var containerSecret string

		if val, exists := inspect.Config.Labels["oops.enable"]; exists && val == "true" {
			isAutoDeploy = true
		}
		if val, exists := inspect.Config.Labels["oops.secret"]; exists {
			containerSecret = val
		}

		if !isAutoDeploy {
			continue
		}

		if action == "image" {
			if !(c.Image == imageURL || strings.Contains(c.Image, imageURL)) {
				continue
			}
		}

		if action == "git" {
			containerGitURL, exists := inspect.Config.Labels["oops.git.url"]
			if !exists || NormalizeGitURL(containerGitURL) != NormalizeGitURL(gitURL) {
				continue
			}
		}

		if re != nil {
			// Container names often start with "/", so we trim it for straightforward regex matching
			cleanName := strings.TrimPrefix(inspect.Name, "/")
			if !re.MatchString(cleanName) {
				continue
			}
		}

		if containerSecret == "" {
			log.Printf("Unauthorized attempt on container %s (missing container secret)", inspect.Name)
			return nil, fmt.Errorf("unauthorized: missing container secret for %s", inspect.Name)
		}
		if token != containerSecret {
			log.Printf("Unauthorized attempt on container %s (invalid container secret)", inspect.Name)
			return nil, fmt.Errorf("unauthorized for container %s", inspect.Name)
		}

		cFiles, hasCFiles := inspect.Config.Labels["com.docker.compose.project.config_files"]
		cPath := ""
		if hasCFiles && cFiles != "" {
			cPaths := strings.Split(cFiles, ",")
			cPath = cPaths[0] // just use the first one
		} else {
			cPath = filepath.Join(inspect.Config.Labels["com.docker.compose.project.working_dir"], "compose.yml")
		}

		targets = append(targets, ResolvedTarget{
			StackName:     inspect.Config.Labels["com.docker.compose.project"],
			ComposePath:   cPath,
			ServiceName:   inspect.Config.Labels["com.docker.compose.service"],
			ContainerName: strings.TrimPrefix(inspect.Name, "/"),
			ContainerID:   c.ID,
			Image:         inspect.Config.Image,
			Labels:        inspect.Config.Labels,
		})
		log.Printf("Validated target container: %s (Name: %s)", c.ID[:10], inspect.Name)
	}

	return targets, nil
}

// ExecuteRecreation pulls the latest image and recreates the target containers
func ExecuteRecreation(ctx context.Context, targets []ResolvedTarget, imageURL string, delayDur time.Duration) error {
	for i, target := range targets {
		log.Printf("Recreating %s...", target.ContainerName)

		if target.ComposePath != "" && target.ServiceName != "" {
			cmdPull := exec.Command("docker", "compose", "-f", target.ComposePath, "pull", target.ServiceName)
			if err := cmdPull.Run(); err != nil {
				log.Printf("Warning: failed to compose pull %s: %v", target.ServiceName, err)
			}

			cmdUp := exec.Command("docker", "compose", "-f", target.ComposePath, "up", "-d", "--no-deps", target.ServiceName)
			if err := cmdUp.Run(); err != nil {
				return fmt.Errorf("failed to recreate %s: %v", target.ServiceName, err)
			}
		} else {
			log.Printf("Warning: No compose info for %s, skipping", target.ContainerName)
		}

		if delayDur > 0 && i < len(targets)-1 {
			time.Sleep(delayDur)
		}
	}

	log.Println("Pruning dangling images...")
	_ = exec.Command("docker", "image", "prune", "-f").Run()

	return nil
}

// ExecuteGitPull creates a temporary container using alpine/git to pull the latest code or checkout a tag, then restarts the target container
func ExecuteGitPull(ctx context.Context, targets []ResolvedTarget, tag string, delayDur time.Duration) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	log.Println("Pulling alpine/git for temporary git operations...")
	out, err := cli.ImagePull(ctx, "alpine/git", image.PullOptions{})
	if err == nil {
		buf := make([]byte, 8192)
		for {
			_, err := out.Read(buf)
			if err != nil {
				break
			}
		}
		out.Close()
	}

	for i, target := range targets {
		id := target.ContainerID
		inspect, err := cli.ContainerInspect(ctx, id)
		if err != nil {
			log.Printf("Error inspecting container %s: %v", id, err)
			continue
		}

		name := inspect.Name

		gitDir, exists := inspect.Config.Labels["oops.git.dir"]
		if !exists || gitDir == "" {
			log.Printf("[%s] Error: Missing 'oops.git.dir' label", name)
			continue
		}

		var gitCmd string
		cleanTag := strings.TrimPrefix(tag, "tags/")
		gitURL := inspect.Config.Labels["oops.git.url"]

		if gitURL != "" {
			log.Printf("[%s] Executing Git Checkout in directory %s (URL: %s, Tag: %s)...", name, gitDir, gitURL, cleanTag)
			gitCmd = fmt.Sprintf(
				"git config --global --add safe.directory %s && "+
					"if [ ! -d \"%s/.git\" ]; then "+
					"  git clone \"%s\" \"%s\"; "+
					"else "+
					"  cd \"%s\" && (git remote set-url origin \"%s\" 2>/dev/null || git remote add origin \"%s\"); "+
					"fi && "+
					"cd \"%s\" && git fetch --all --tags --force && git checkout -f tags/%s",
				gitDir, gitDir, gitURL, gitDir, gitDir, gitURL, gitURL, gitDir, cleanTag,
			)
		} else {
			log.Printf("[%s] Executing Git Checkout in directory %s (Tag: %s)...", name, gitDir, cleanTag)
			gitCmd = fmt.Sprintf("git config --global --add safe.directory %s && cd %s && git fetch --all --tags --force && git checkout -f tags/%s", gitDir, gitDir, cleanTag)
		}

		created, err := cli.ContainerCreate(
			ctx,
			&container.Config{
				Image:      "alpine/git",
				Entrypoint: []string{"sh", "-c"},
				Cmd:        []string{gitCmd},
			},
			&container.HostConfig{
				VolumesFrom: []string{id},
				AutoRemove:  true,
			},
			nil,
			nil,
			"",
		)
		if err != nil {
			log.Printf("[%s] Error creating git container: %v", name, err)
			continue
		}

		if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
			log.Printf("[%s] Error starting git container: %v", name, err)
			continue
		}

		statusCh, errCh := cli.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("[%s] Error waiting for git container: %v", name, err)
			}
		case status := <-statusCh:
			if status.StatusCode != 0 {
				log.Printf("[%s] Warning: Git Pull failed (Exit Code: %d)", name, status.StatusCode)
			} else {
				log.Printf("[%s] Git Pull completed successfully", name)
			}
		}

		toolImage := inspect.Config.Labels["oops.tool.image"]
		toolCmd := inspect.Config.Labels["oops.tool.cmd"]

		if toolImage != "" && toolCmd != "" {
			log.Printf("[%s] Pulling tool image: %s ...", name, toolImage)
			outTool, err := cli.ImagePull(ctx, toolImage, image.PullOptions{})
			if err == nil {
				buf := make([]byte, 8192)
				for {
					if _, err := outTool.Read(buf); err != nil {
						break
					}
				}
				outTool.Close()
			}

			log.Printf("[%s] Executing tool command: %s ...", name, toolCmd)
			toolCreated, err := cli.ContainerCreate(
				ctx,
				&container.Config{
					Image:      toolImage,
					Entrypoint: []string{"sh", "-c"},
					Cmd:        []string{toolCmd},
					WorkingDir: gitDir,
				},
				&container.HostConfig{
					VolumesFrom: []string{id},
					AutoRemove:  true,
				},
				nil,
				nil,
				"",
			)

			if err != nil {
				log.Printf("[%s] Error creating tool container: %v", name, err)
			} else {
				if err := cli.ContainerStart(ctx, toolCreated.ID, container.StartOptions{}); err != nil {
					log.Printf("[%s] Error starting tool container: %v", name, err)
				} else {
					toolStatusCh, toolErrCh := cli.ContainerWait(ctx, toolCreated.ID, container.WaitConditionNotRunning)
					select {
					case err := <-toolErrCh:
						if err != nil {
							log.Printf("[%s] Error waiting for tool container: %v", name, err)
						}
					case status := <-toolStatusCh:
						if status.StatusCode != 0 {
							log.Printf("[%s] Warning: Tool command failed (Exit Code: %d)", name, status.StatusCode)
						} else {
							log.Printf("[%s] Tool command completed successfully", name)
						}
					}
				}
			}
		}

		log.Printf("[%s] Restarting target container...", name)
		if target.ComposePath != "" && target.ServiceName != "" {
			cmdRestart := exec.Command("docker", "compose", "-f", target.ComposePath, "restart", target.ServiceName)
			if err := cmdRestart.Run(); err != nil {
				log.Printf("[%s] Error restarting container via compose: %v", name, err)
			} else {
				log.Printf("[%s] Container restarted successfully", name)
			}
		} else {
			log.Printf("[%s] Warning: No compose info, unable to restart via compose CLI", name)
		}

		if delayDur > 0 && i < len(targets)-1 {
			time.Sleep(delayDur)
		}
	}

	return nil
}

// CountAllContainers returns the total number of containers currently on the Docker daemon.
func CountAllContainers(ctx context.Context) (int, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %v", err)
	}
	return len(containers), nil
}

// WipeAllContainers forcefully stops and removes all containers on the Docker daemon.
func WipeAllContainers(ctx context.Context) (int, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %v", err)
	}

	count := 0
	for _, c := range containers {
		err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: false,
		})
		if err != nil {
			log.Printf("Warning: failed to remove container %s: %v", c.ID[:12], err)
			continue
		}
		count++
	}

	return count, nil
}
