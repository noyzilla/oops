package docker

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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
func ValidateAndFindTargets(ctx context.Context, action, imageURL, gitURL, containerRegex, token string) ([]string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var targetIDs []string

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

		targetIDs = append(targetIDs, c.ID)
		log.Printf("Validated target container: %s (Name: %s)", c.ID[:10], inspect.Name)
	}

	return targetIDs, nil
}

// ExecuteRecreation pulls the latest image and recreates the target containers
func ExecuteRecreation(ctx context.Context, targetIDs []string, imageURL string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %v", err)
	}
	defer cli.Close()

	if imageURL != "" {
		log.Printf("Pulling latest image: %s", imageURL)
		out, err := cli.ImagePull(ctx, imageURL, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %v", err)
		}

		buf := make([]byte, 8192)
		for {
			_, err := out.Read(buf)
			if err != nil {
				break
			}
		}
		out.Close()
		log.Printf("Successfully pulled image: %s", imageURL)
	}

	for _, id := range targetIDs {
		if err := recreateSingleContainer(ctx, cli, id); err != nil {
			log.Printf("Error recreating container %s: %v", id, err)
		}
	}

	log.Println("Pruning dangling images...")
	pruneReport, err := cli.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	if err != nil {
		log.Printf("Warning: failed to prune images: %v", err)
	} else {
		log.Printf("Pruned images, reclaimed space: %d bytes", pruneReport.SpaceReclaimed)
	}

	return nil
}

// recreateSingleContainer handles the graceful shutdown, removal, and recreation of a single container
func recreateSingleContainer(ctx context.Context, cli *client.Client, id string) error {
	inspect, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}

	name := inspect.Name
	log.Printf("Recreating container %s...", name)

	var stopCmd string
	stopTimeout := 60

	if val, exists := inspect.Config.Labels["oops.stop.cmd"]; exists {
		stopCmd = val
	}
	if val, exists := inspect.Config.Labels["oops.stop.timeout"]; exists {
		if t, err := strconv.Atoi(val); err == nil {
			stopTimeout = t
		}
	}

	if stopCmd != "" {
		log.Printf("[%s] Executing stop command: %s (Timeout: %ds)", name, stopCmd, stopTimeout)

		execConfig := container.ExecOptions{
			Cmd: []string{"sh", "-c", stopCmd},
		}

		execID, err := cli.ContainerExecCreate(ctx, id, execConfig)
		if err != nil {
			log.Printf("[%s] Warning: Failed to create exec command: %v", name, err)
		} else {
			if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
				log.Printf("[%s] Warning: Failed to start exec command: %v", name, err)
			} else {
				startWait := time.Now()
				for {
					if time.Since(startWait).Seconds() > float64(stopTimeout) {
						log.Printf("[%s] Warning: Stop command timed out after %ds", name, stopTimeout)
						break
					}

					execInspect, err := cli.ContainerExecInspect(ctx, execID.ID)
					if err != nil {
						log.Printf("[%s] Warning: Error inspecting exec command: %v", name, err)
						break
					}

					if !execInspect.Running {
						log.Printf("[%s] Stop command completed (Exit Code: %d)", name, execInspect.ExitCode)
						break
					}
					time.Sleep(1 * time.Second)
				}
			}
		}
	}

	log.Printf("Stopping %s...", name)
	timeout := 10
	stopOptions := container.StopOptions{Timeout: &timeout}
	if err := cli.ContainerStop(ctx, id, stopOptions); err != nil {
		return fmt.Errorf("failed to stop container: %v", err)
	}

	log.Printf("Removing %s...", name)
	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %v", err)
	}

	log.Printf("Creating %s...", name)

	hostConfig := inspect.HostConfig

	created, err := cli.ContainerCreate(
		ctx,
		inspect.Config,
		hostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		},
		nil,
		name,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	log.Printf("Starting %s...", name)
	if err := cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start new container: %v", err)
	}

	log.Printf("Successfully recreated container %s (New ID: %s)", name, created.ID[:10])
	return nil
}

// ExecuteGitPull creates a temporary container using alpine/git to pull the latest code or checkout a tag, then restarts the target container
func ExecuteGitPull(ctx context.Context, targetIDs []string, tag string) error {
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

	for _, id := range targetIDs {
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
		timeout := 10
		stopOptions := container.StopOptions{Timeout: &timeout}
		if err := cli.ContainerRestart(ctx, id, stopOptions); err != nil {
			log.Printf("[%s] Error restarting container: %v", name, err)
		} else {
			log.Printf("[%s] Container restarted successfully", name)
		}
	}

	return nil
}
