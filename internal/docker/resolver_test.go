package docker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noyzilla/oops/internal/docker"
)

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern   string
		candidate string
		expected  bool
	}{
		// Exact match
		{"mysql", "mysql", true},
		{"mysql", "postgres", false},

		// Double Dot Prefix
		{"app..", "app1", true},
		{"app..", "app-web", true},
		{"app..", "web-app", false},

		// Double Dot Suffix
		{"..worker", "email-worker", true},
		{"..worker", "worker", true},
		{"..worker", "worker-email", false},

		// Double Dot Contains
		{"..api..", "web-api-service", true},
		{"..api..", "api", true},
		{"..api..", "web-backend", false},
	}

	for _, tt := range tests {
		got := docker.MatchWildcard(tt.pattern, tt.candidate)
		if got != tt.expected {
			t.Errorf("MatchWildcard(%q, %q) = %v, expected %v", tt.pattern, tt.candidate, got, tt.expected)
		}
	}
}

func TestResolveTargetsMultiLayer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-resolver-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create layers: edge, db, apps
	edgeDir := filepath.Join(tmpDir, "edge")
	dbDir := filepath.Join(tmpDir, "db")
	appsDir := filepath.Join(tmpDir, "apps")

	os.MkdirAll(edgeDir, 0755)
	os.MkdirAll(dbDir, 0755)
	os.MkdirAll(appsDir, 0755)

	os.WriteFile(filepath.Join(edgeDir, "docker-compose.yml"), []byte(`
services:
  caddy:
    image: caddy:latest
`), 0644)

	os.WriteFile(filepath.Join(dbDir, "docker-compose.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  redis:
    image: redis:alpine
`), 0644)

	os.WriteFile(filepath.Join(appsDir, "docker-compose.yml"), []byte(`
services:
  app-web:
    image: myapp:web
    labels:
      - "oops.stop.cmd=sleep 2"
      - "oops.stop.timeout=10"
  app-worker:
    image: myapp:worker
`), 0644)

	// 1. Resolve all (empty targets) -> dependency order edge -> db -> apps
	all, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error resolving all: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 targets, got %d", len(all))
	}
	if all[0].ServiceName != "caddy" || all[1].ServiceName != "mysql" || all[2].ServiceName != "redis" {
		t.Errorf("unexpected ordering for all targets: %+v", all)
	}

	// 2. Resolve layer target /apps
	apps, err := docker.ResolveTargets(tmpDir, []string{"/apps"})
	if err != nil {
		t.Fatalf("unexpected error resolving /apps: %v", err)
	}
	if len(apps) != 2 || apps[0].ServiceName != "app-web" || apps[1].ServiceName != "app-worker" {
		t.Errorf("unexpected results for /apps: %+v", apps)
	}

	// 3. Resolve scoped service /db/mysql
	mysqlTarget, err := docker.ResolveTargets(tmpDir, []string{"/db/mysql"})
	if err != nil {
		t.Fatalf("unexpected error resolving /db/mysql: %v", err)
	}
	if len(mysqlTarget) != 1 || mysqlTarget[0].ServiceName != "mysql" {
		t.Errorf("unexpected results for /db/mysql: %+v", mysqlTarget)
	}

	// 4. Resolve Double Dot wildcard app..
	wildcardTargets, err := docker.ResolveTargets(tmpDir, []string{"app.."})
	if err != nil {
		t.Fatalf("unexpected error resolving app..: %v", err)
	}
	if len(wildcardTargets) != 2 {
		t.Fatalf("expected 2 targets for app.., got %d", len(wildcardTargets))
	}

	// Check labels parsing
	if wildcardTargets[0].Labels["oops.stop.cmd"] != "sleep 2" {
		t.Errorf("expected label oops.stop.cmd to be 'sleep 2', got %q", wildcardTargets[0].Labels["oops.stop.cmd"])
	}
}
