package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// GetHealthcheckTimeout reads OOPS_HEALTHCHECK_TIMEOUT (default 10m)
func GetHealthcheckTimeout() time.Duration {
	val := os.Getenv("OOPS_HEALTHCHECK_TIMEOUT")
	return ParseDurationWithDefault(val, 10*time.Minute)
}

// GetHealthcheckInterval reads OOPS_HEALTHCHECK_INTERVAL (default 3s)
func GetHealthcheckInterval() time.Duration {
	val := os.Getenv("OOPS_HEALTHCHECK_INTERVAL")
	return ParseDurationWithDefault(val, 3*time.Second)
}

// ParseDelay parses standard duration strings (5s, 10s, 1m) or pure integers (5 -> 5s)
func ParseDelay(delayStr string) (time.Duration, error) {
	trimmed := strings.TrimSpace(delayStr)
	if trimmed == "" || trimmed == "0" || trimmed == "0s" {
		return 0, nil
	}

	// If integer without unit, treat as seconds
	if sec, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(sec) * time.Second, nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid delay duration %q: %w", delayStr, err)
	}
	return d, nil
}

// CheckContainerHealth evaluates the health of a container using Docker Native Health or HTTP fallback
func CheckContainerHealth(ctx context.Context, cli *client.Client, containerID string, healthURL string) (bool, string, error) {
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, "", fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	// 1. Docker Native Health Check
	if inspect.State != nil && inspect.State.Health != nil {
		status := inspect.State.Health.Status
		if status == container.Healthy {
			return true, status, nil
		}
		if status == container.Unhealthy {
			return false, status, fmt.Errorf("container is unhealthy")
		}
		return false, status, nil
	}

	// 2. Custom oops.health.url fallback
	if healthURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			return false, "http_error", err
		}

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return true, "healthy_http", nil
			}
		}
		return false, "http_waiting", nil
	}

	// If no healthcheck defined and container is running -> treat as healthy
	if inspect.State != nil && inspect.State.Running {
		return true, "running", nil
	}

	return false, "stopped", nil
}

// WaitForHealth polls container health until healthy, unhealthy, or timed out
func WaitForHealth(ctx context.Context, cli *client.Client, containerID string, healthURL string, timeout time.Duration, interval time.Duration) error {
	if timeout <= 0 {
		timeout = GetHealthcheckTimeout()
	}
	if interval <= 0 {
		interval = GetHealthcheckInterval()
	}

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("healthcheck timed out after %v", timeout)
		}

		healthy, status, err := CheckContainerHealth(ctx, cli, containerID, healthURL)
		if err != nil && status == container.Unhealthy {
			return err
		}
		if healthy {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
