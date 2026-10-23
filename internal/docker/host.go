package docker

import (
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
)

// dockerContextInspector abstracts inspecting the active docker context endpoint for testing.
var dockerContextInspector = defaultDockerContextInspect

func defaultDockerContextInspect() (string, error) {
	out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveDockerHost resolves the active Docker host endpoint in the following order:
// 1. $DOCKER_HOST if explicitly set and non-empty.
// 2. Active Docker context endpoint via `docker context inspect --format '{{.Endpoints.docker.Host}}'`.
// 3. Empty string (falls back to client.DefaultDockerHost).
func ResolveDockerHost() string {
	if h := strings.TrimSpace(os.Getenv("DOCKER_HOST")); h != "" {
		return h
	}
	if host, err := dockerContextInspector(); err == nil && host != "" {
		return host
	}
	return ""
}

// NewClient creates a new Docker API client using the resolved Docker host and API version negotiation.
func NewClient(opts ...client.Opt) (*client.Client, error) {
	defaultOpts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if host := ResolveDockerHost(); host != "" {
		defaultOpts = append(defaultOpts, client.WithHost(host))
	}
	allOpts := append(defaultOpts, opts...)
	return client.NewClientWithOpts(allOpts...)
}

