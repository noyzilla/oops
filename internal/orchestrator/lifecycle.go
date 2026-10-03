package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ExecutePreStopHook executes container's pre-stop command if defined in oops.stop.cmd label
func ExecutePreStopHook(ctx context.Context, cli *client.Client, containerID string, stopCmd string, timeoutSeconds int) error {
	if stopCmd == "" {
		return nil
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	name := inspect.Name
	log.Printf("[%s] Executing pre-stop hook: %q (Timeout: %ds)", name, stopCmd, timeoutSeconds)

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", stopCmd},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		log.Printf("[%s] Warning: Failed to create exec command: %v", name, err)
		return nil
	}

	if err := cli.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		log.Printf("[%s] Warning: Failed to start exec command: %v", name, err)
		return nil
	}

	startWait := time.Now()
	for {
		if time.Since(startWait).Seconds() > float64(timeoutSeconds) {
			log.Printf("[%s] Warning: Pre-stop hook timed out after %ds", name, timeoutSeconds)
			break
		}

		execInspect, err := cli.ContainerExecInspect(ctx, execID.ID)
		if err != nil {
			log.Printf("[%s] Warning: Error inspecting exec command: %v", name, err)
			break
		}

		if !execInspect.Running {
			log.Printf("[%s] Pre-stop hook finished (Exit Code: %d)", name, execInspect.ExitCode)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// GetStopTimeout parses timeout from labels or default
func GetStopTimeout(labels map[string]string) int {
	if val, ok := labels["oops.stop.timeout"]; ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			return t
		}
	}
	return 30
}
