package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// ComposeCommandEnv returns the process environment with COMPOSE_IGNORE_ORPHANS=True,
// and ensures UID and GID of current caller are present so compose variable expansion
// (e.g. user: "${UID:-1000}:${GID:-1000}") evaluates correctly without defaulting.
func ComposeCommandEnv() []string {
	env := os.Environ()
	hasUID := false
	hasGID := false
	for _, e := range env {
		if strings.HasPrefix(e, "UID=") {
			hasUID = true
		}
		if strings.HasPrefix(e, "GID=") {
			hasGID = true
		}
	}
	if !hasUID {
		env = append(env, fmt.Sprintf("UID=%d", os.Getuid()))
	}
	if !hasGID {
		env = append(env, fmt.Sprintf("GID=%d", os.Getgid()))
	}
	return append(env, "COMPOSE_IGNORE_ORPHANS=True")
}

// SyncUserUIDGIDToEnvFile updates or appends UID and GID of the current OS caller in the given .env file.
func SyncUserUIDGIDToEnvFile(envPath string) error {
	uid := os.Getuid()
	gid := os.Getgid()

	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			newContent := fmt.Sprintf("UID=%d\nGID=%d\n", uid, gid)
			return os.WriteFile(envPath, []byte(newContent), 0644)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	foundUID := false
	foundGID := false

	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "UID=") {
			newLines = append(newLines, fmt.Sprintf("UID=%d", uid))
			foundUID = true
		} else if strings.HasPrefix(trimmed, "GID=") {
			newLines = append(newLines, fmt.Sprintf("GID=%d", gid))
			foundGID = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// Remove trailing empty line if present before appending
	if len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	if !foundUID {
		newLines = append(newLines, fmt.Sprintf("UID=%d", uid))
	}
	if !foundGID {
		newLines = append(newLines, fmt.Sprintf("GID=%d", gid))
	}
	newLines = append(newLines, "") // trailing newline

	updated := strings.Join(newLines, "\n")
	if updated == string(content) {
		return nil // No change needed
	}

	return os.WriteFile(envPath, []byte(updated), 0644)
}

// RunComposeFiles executes docker compose with multi -f files and auto-detected .env
func RunComposeFiles(composeFiles []string, args ...string) error {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "compose")

	if len(composeFiles) > 0 {
		if envFile := FindEnvFile(composeFiles[0]); envFile != "" {
			_ = SyncUserUIDGIDToEnvFile(envFile)
			cmdArgs = append(cmdArgs, "--env-file", envFile)
		}
		for _, f := range composeFiles {
			cmdArgs = append(cmdArgs, "-f", f)
		}
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("docker", cmdArgs...)
	cmd.Env = ComposeCommandEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %v failed: %s (%w)", cmdArgs, string(out), err)
	}
	log.Printf("%s", string(out))
	return nil
}

// RunComposeCommand executes docker compose with specific arguments and auto-detected .env
func RunComposeCommand(composePath string, args ...string) error {
	return RunComposeFiles([]string{composePath}, args...)
}

func (o *Orchestrator) runTargetCompose(target docker.ResolvedTarget, args ...string) error {
	if len(target.ComposeFiles) > 0 {
		return RunComposeFiles(target.ComposeFiles, args...)
	}
	return RunComposeCommand(target.ComposePath, args...)
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
		if err := o.runTargetCompose(target, "pull", target.ServiceName); err != nil {
			log.Printf("Warning: compose pull failed: %v", err)
		}

		// Step 2 - Recreate service container
		log.Printf("[%s] Recreating container with --no-deps...", target.ServiceName)
		if err := o.runTargetCompose(target, "up", "-d", "--no-deps", target.ServiceName); err != nil {
			return fmt.Errorf("failed to recreate %s: %w", target.ServiceName, err)
		}

		// Step 3 - Health Polling
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

		if err := o.runTargetCompose(target, "stop", target.ServiceName); err != nil {
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

		if err := o.runTargetCompose(target, "restart", target.ServiceName); err != nil {
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
	type executionGroup struct {
		stackName    string
		composeFiles []string
		services     []string
	}

	var groups []executionGroup
	for _, t := range targets {
		cFiles := t.ComposeFiles
		if len(cFiles) == 0 && t.ComposePath != "" {
			cFiles = []string{t.ComposePath}
		}

		if len(groups) > 0 && groups[len(groups)-1].stackName == t.StackName {
			groups[len(groups)-1].services = append(groups[len(groups)-1].services, t.ServiceName)
		} else {
			groups = append(groups, executionGroup{
				stackName:    t.StackName,
				composeFiles: cFiles,
				services:     []string{t.ServiceName},
			})
		}
	}

	for i, group := range groups {
		log.Printf("==> Starting stack /%s (Services: %v)...", group.stackName, group.services)
		cmdArgs := append([]string{"up", "-d"}, group.services...)
		if err := RunComposeFiles(group.composeFiles, cmdArgs...); err != nil {
			return err
		}

		if delay > 0 && i < len(groups)-1 {
			log.Printf("Pausing %v before starting next stack...", delay)
			time.Sleep(delay)
		}
	}
	return nil
}
