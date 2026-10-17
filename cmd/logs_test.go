package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/noyzilla/oops/internal/docker"
)

func TestBuildComposeLogsArgs_MultiStackAndTailChaining(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("OOPS_SECRET=test\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`x-oops-stack:
  - /edge
  - /db
networks:
  default:
    name: oops_default
`), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yaml"), []byte(`x-oops-stack:
  - caddy-proxy
  - oops
services:
  caddy-proxy:
    image: lucaslorentz/caddy-docker-proxy:alpine
  oops:
    image: ghcr.io/noyzilla/oops:latest
`), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`x-oops-stack:
  - mysql
  - redis
services:
  mysql:
    image: mysql:8.4
  redis:
    image: redis:7-alpine
`), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.local.yaml"), []byte(`services: {}
`), 0644)

	targets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("ResolveTargets failed: %v", err)
	}

	composeArgs, services := buildComposeLogsArgs(tmpDir, targets, 50, true)

	expectedServices := []string{"caddy-proxy", "oops", "mysql", "redis"}
	if !reflect.DeepEqual(services, expectedServices) {
		t.Errorf("expected services %v, got %v", expectedServices, services)
	}

	expectedArgs := []string{
		"compose",
		"--env-file", filepath.Join(tmpDir, ".env"),
		"-f", filepath.Join(tmpDir, "compose.yaml"),
		"-f", filepath.Join(tmpDir, "compose.edge.yaml"),
		"-f", filepath.Join(tmpDir, "compose.db.yaml"),
		"-f", filepath.Join(tmpDir, "compose.local.yaml"),
		"logs",
		"--tail", "50",
		"-f",
		"caddy-proxy", "oops", "mysql", "redis",
	}
	if !reflect.DeepEqual(composeArgs, expectedArgs) {
		t.Errorf("expected composeArgs:\n  %v\ngot:\n  %v", expectedArgs, composeArgs)
	}
}

