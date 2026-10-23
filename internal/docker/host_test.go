package docker

import (
	"errors"
	"os"
	"testing"
)

func TestResolveDockerHost_EnvPrecedence(t *testing.T) {
	oldEnv := os.Getenv("DOCKER_HOST")
	defer os.Setenv("DOCKER_HOST", oldEnv)

	os.Setenv("DOCKER_HOST", "unix:///custom/env/docker.sock")

	oldInspector := dockerContextInspector
	defer func() { dockerContextInspector = oldInspector }()
	dockerContextInspector = func() (string, error) {
		return "unix:///context/docker.sock", nil
	}

	got := ResolveDockerHost()
	if got != "unix:///custom/env/docker.sock" {
		t.Fatalf("expected env host to take precedence, got %q", got)
	}
}

func TestResolveDockerHost_ContextFallback(t *testing.T) {
	oldEnv := os.Getenv("DOCKER_HOST")
	defer os.Setenv("DOCKER_HOST", oldEnv)

	os.Unsetenv("DOCKER_HOST")

	oldInspector := dockerContextInspector
	defer func() { dockerContextInspector = oldInspector }()
	dockerContextInspector = func() (string, error) {
		return "unix:///Users/tester/.orbstack/run/docker.sock", nil
	}

	got := ResolveDockerHost()
	if got != "unix:///Users/tester/.orbstack/run/docker.sock" {
		t.Fatalf("expected context host fallback, got %q", got)
	}
}

func TestResolveDockerHost_InspectorError(t *testing.T) {
	oldEnv := os.Getenv("DOCKER_HOST")
	defer os.Setenv("DOCKER_HOST", oldEnv)

	os.Unsetenv("DOCKER_HOST")

	oldInspector := dockerContextInspector
	defer func() { dockerContextInspector = oldInspector }()
	dockerContextInspector = func() (string, error) {
		return "", errors.New("command not found")
	}

	got := ResolveDockerHost()
	if got != "" {
		t.Fatalf("expected empty host on inspector error, got %q", got)
	}
}

func TestNewClient(t *testing.T) {
	oldEnv := os.Getenv("DOCKER_HOST")
	defer os.Setenv("DOCKER_HOST", oldEnv)
	os.Unsetenv("DOCKER_HOST")

	oldInspector := dockerContextInspector
	defer func() { dockerContextInspector = oldInspector }()
	dockerContextInspector = func() (string, error) {
		return "unix:///var/run/docker.sock", nil
	}

	cli, err := NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
	_ = cli.Close()
}

