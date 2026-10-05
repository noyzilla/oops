package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ParseDurationWithDefault parses duration strings (30s, 10m, 1h) or numeric seconds (30, 600)
// If the input is empty or invalid, it returns fallback duration.
func ParseDurationWithDefault(raw string, defaultDur time.Duration) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultDur
	}
	if sec, err := strconv.Atoi(trimmed); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if d, err := time.ParseDuration(trimmed); err == nil && d >= 0 {
		return d
	}
	return defaultDur
}

// GetDefaultStopTimeout reads OOPS_STOP_TIMEOUT with 30s default
func GetDefaultStopTimeout() time.Duration {
	val := os.Getenv("OOPS_STOP_TIMEOUT")
	return ParseDurationWithDefault(val, 30*time.Second)
}

// GetStopTimeout parses timeout from labels or default
func GetStopTimeout(labels map[string]string) time.Duration {
	defaultTimeout := GetDefaultStopTimeout()
	if val, ok := labels["oops.stop.timeout"]; ok && val != "" {
		return ParseDurationWithDefault(val, defaultTimeout)
	}
	return defaultTimeout
}

// ExecutePreStopHook executes container's pre-stop command if defined in oops.stop.cmd label
func ExecutePreStopHook(ctx context.Context, cli *client.Client, containerID string, stopCmd string, timeout time.Duration) error {
	if stopCmd == "" {
		return nil
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	name := inspect.Name
	log.Printf("[%s] Executing pre-stop hook: %q (Timeout: %v)", name, stopCmd, timeout)

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
		if time.Since(startWait) > timeout {
			log.Printf("[%s] Warning: Pre-stop hook timed out after %v", name, timeout)
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
