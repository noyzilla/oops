package db

import (
	"context"
	"os/exec"
	"strings"
)

// InspectContainerEnv extracts an environment variable from a running container.
func InspectContainerEnv(ctx context.Context, containerName, envKey string) string {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{range .Config.Env}}{{println .}}{{end}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(out), "\n")
	prefix := envKey + "="
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}
