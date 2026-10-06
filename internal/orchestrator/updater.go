package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/docker"
)

// Orchestrator coordinates compose operations with Docker API and lifecycle hooks
type Orchestrator struct {
	dockerCli *client.Client
	// WorkDir is the oopsbox root used by the storage guard; empty disables the guard
	WorkDir string
}

// New creates a new Orchestrator instance
func New() (*Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize docker client: %w", err)
	}
	return &Orchestrator{dockerCli: cli}, nil
}

// Close releases the docker client
func (o *Orchestrator) Close() error {
	if o.dockerCli != nil {
		return o.dockerCli.Close()
	}
	return nil
}

// FindContainerID returns the running container ID matching a service name in a compose layer
func (o *Orchestrator) FindContainerID(ctx context.Context, target docker.ResolvedTarget) (string, error) {
	containers, err := o.dockerCli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		serviceLabel := c.Labels["com.docker.compose.service"]
		if serviceLabel == target.ServiceName {
			return c.ID, nil
		}
		if target.ContainerName != "" {
			for _, name := range c.Names {
				if name == "/"+target.ContainerName || name == target.ContainerName {
					return c.ID, nil
				}
			}
		}
	}

	return "", nil
}

// FindEnvFile walks up from composePath to locate the nearest .env file in the workspace
func FindEnvFile(composePath string) string {
	dir := filepath.Dir(composePath)
	if dir == "." || dir == "" {
		if abs, err := filepath.Abs(composePath); err == nil {
			dir = filepath.Dir(abs)
		}
	} else if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}

	curr := dir
	for {
		candidate := filepath.Join(curr, ".env")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}

// RunComposeCommand executes docker compose with specific arguments and auto-detected .env
func RunComposeCommand(composePath string, args ...string) error {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "compose")

	if envFile := FindEnvFile(composePath); envFile != "" {
		cmdArgs = append(cmdArgs, "--env-file", envFile)
	}

	cmdArgs = append(cmdArgs, "-f", composePath)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("docker", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %v failed: %s (%w)", cmdArgs, string(out), err)
	}
	log.Printf("%s", string(out))
	return nil
}

// Update executes sequential rolling updates across matched targets
func (o *Orchestrator) Update(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	targets, guardErr := o.applyGuard(targets)
	if err := o.updateAllowed(ctx, targets, delay); err != nil {
		return err
	}
	return guardErr
}

func (o *Orchestrator) updateAllowed(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	for i, target := range targets {
		log.Printf("==> [%d/%d] Rolling update service %s (Stack: %s)...", i+1, len(targets), target.ServiceName, target.StackName)

		// Step 1 - Pull latest image
		log.Printf("[%s] Pulling image for %s...", target.ServiceName, target.ServiceName)
		if err := RunComposeCommand(target.ComposePath, "pull", target.ServiceName); err != nil {
			log.Printf("Warning: compose pull failed: %v", err)
		}

		// Step 2 - Execute Pre-Stop Hook if container is running
		cID, err := o.FindContainerID(ctx, target)
		if err == nil && cID != "" {
			stopCmd := target.Labels["oops.stop.cmd"]
			stopTimeout := GetStopTimeout(target.Labels)
			if stopCmd != "" {
				if err := ExecutePreStopHook(ctx, o.dockerCli, cID, stopCmd, stopTimeout); err != nil {
					log.Printf("[%s] Pre-stop hook warning: %v", target.ServiceName, err)
				}
			}
		}

		// Step 3 - Recreate service container
		log.Printf("[%s] Recreating container with --no-deps...", target.ServiceName)
		if err := RunComposeCommand(target.ComposePath, "up", "-d", "--no-deps", target.ServiceName); err != nil {
			return fmt.Errorf("failed to recreate %s: %w", target.ServiceName, err)
		}

		// Step 4 - Health Polling
		newCID, err := o.FindContainerID(ctx, target)
		if err != nil || newCID == "" {
			log.Printf("[%s] Warning: could not resolve new container ID for healthcheck", target.ServiceName)
		} else {
			healthURL := target.Labels["oops.health.url"]
			log.Printf("[%s] Polling health status (Health URL: %s)...", target.ServiceName, healthURL)
			if err := WaitForHealth(ctx, o.dockerCli, newCID, healthURL, 0, 0); err != nil {
				return fmt.Errorf("service %s health check failed: %w", target.ServiceName, err)
			}
			log.Printf("[%s] Service is healthy!", target.ServiceName)
		}

		// Inter-service delay
		if delay > 0 && i < len(targets)-1 {
			log.Printf("Pausing %v before next service update...", delay)
			time.Sleep(delay)
		}
	}

	return nil
}

// Stop executes graceful stopping across matched targets sequentially
func (o *Orchestrator) Stop(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	for i, target := range targets {
		log.Printf("==> [%d/%d] Stopping service %s (Stack: %s)...", i+1, len(targets), target.ServiceName, target.StackName)

		cID, err := o.FindContainerID(ctx, target)
		if err == nil && cID != "" {
			stopCmd := target.Labels["oops.stop.cmd"]
			stopTimeout := GetStopTimeout(target.Labels)
			if stopCmd != "" {
				_ = ExecutePreStopHook(ctx, o.dockerCli, cID, stopCmd, stopTimeout)
			}
		}

		if err := RunComposeCommand(target.ComposePath, "stop", target.ServiceName); err != nil {
			log.Printf("Warning: failed to stop %s: %v", target.ServiceName, err)
		}

		if delay > 0 && i < len(targets)-1 {
			log.Printf("Pausing %v before stopping next service...", delay)
			time.Sleep(delay)
		}
	}
	return nil
}

// Restart executes graceful restart across matched targets sequentially
func (o *Orchestrator) Restart(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	targets, guardErr := o.applyGuard(targets)
	if err := o.restartAllowed(ctx, targets, delay); err != nil {
		return err
	}
	return guardErr
}

func (o *Orchestrator) restartAllowed(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	for i, target := range targets {
		log.Printf("==> [%d/%d] Restarting service %s (Stack: %s)...", i+1, len(targets), target.ServiceName, target.StackName)

		cID, err := o.FindContainerID(ctx, target)
		if err == nil && cID != "" {
			stopCmd := target.Labels["oops.stop.cmd"]
			stopTimeout := GetStopTimeout(target.Labels)
			if stopCmd != "" {
				_ = ExecutePreStopHook(ctx, o.dockerCli, cID, stopCmd, stopTimeout)
			}
		}

		if err := RunComposeCommand(target.ComposePath, "restart", target.ServiceName); err != nil {
			log.Printf("Warning: failed to restart %s: %v", target.ServiceName, err)
		}

		if delay > 0 && i < len(targets)-1 {
			log.Printf("Pausing %v before restarting next service...", delay)
			time.Sleep(delay)
		}
	}
	return nil
}

// Up starts matched targets grouped by stack
func (o *Orchestrator) Up(ctx context.Context, targets []docker.ResolvedTarget, delay time.Duration) error {
	targets, guardErr := o.applyGuard(targets)
	if err := o.upAllowed(targets, delay); err != nil {
		return err
	}
	return guardErr
}

func (o *Orchestrator) upAllowed(targets []docker.ResolvedTarget, delay time.Duration) error {
	stackServiceMap := make(map[string][]string)
	stackComposeMap := make(map[string]string)
	var orderedStacks []string

	for _, t := range targets {
		if _, exists := stackServiceMap[t.StackName]; !exists {
			orderedStacks = append(orderedStacks, t.StackName)
			stackComposeMap[t.StackName] = t.ComposePath
		}
		stackServiceMap[t.StackName] = append(stackServiceMap[t.StackName], t.ServiceName)
	}

	for i, stack := range orderedStacks {
		services := stackServiceMap[stack]
		composePath := stackComposeMap[stack]

		log.Printf("==> Starting stack /%s (Services: %v)...", stack, services)
		cmdArgs := append([]string{"up", "-d"}, services...)
		if err := RunComposeCommand(composePath, cmdArgs...); err != nil {
			return err
		}

		if delay > 0 && i < len(orderedStacks)-1 {
			log.Printf("Pausing %v before starting next stack...", delay)
			time.Sleep(delay)
		}
	}
	return nil
}
