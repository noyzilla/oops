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

func TestResolveTargetsMultiStack(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-resolver-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create flat stacks at root
	os.WriteFile(filepath.Join(tmpDir, "docker-compose.edge.yml"), []byte(`
services:
  caddy:
    image: caddy:latest
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "docker-compose.db.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  redis:
    image: redis:alpine
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(`
services:
  app-web:
    image: myapp:web
    labels:

  app-worker:
    image: myapp:worker
`), 0644)

	// 1. Resolve all (empty targets) -> dependency order edge -> db -> apps (.)
	all, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error resolving all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 targets, got %d: %+v", len(all), all)
	}
	if all[0].ServiceName != "app-web" || all[1].ServiceName != "app-worker" {
		t.Errorf("unexpected targets: %+v", all)
	}

	// 2. Resolve stack target /db
	dbTargets, err := docker.ResolveTargets(tmpDir, []string{"/db"})
	if err != nil {
		t.Fatalf("unexpected error resolving /db: %v", err)
	}
	// /db depends on /edge, so edge should be booted first
	if len(dbTargets) != 2 {
		t.Fatalf("expected 2 targets for /db, got %d", len(dbTargets))
	}
	if dbTargets[0].ServiceName != "mysql" || dbTargets[1].ServiceName != "redis" {
		t.Errorf("unexpected results for /db: %+v", dbTargets)
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
	// app.. resolves to app-web and app-worker, which are in stack `.`.
	// `.` depends on /db which depends on /edge. So we should get 1 (edge) + 2 (db) + 2 (apps) = 5 targets.
	if len(wildcardTargets) != 2 {
		t.Fatalf("expected 2 targets for app.., got %d", len(wildcardTargets))
	}

	// Check labels parsing for app-web
}

func TestResolveImageAliasesAndRegistryShortcuts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-img-alias-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "compose.apps.yml"), []byte(`
services:
  web-api:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  worker:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  frontend:
    image: ghcr.io/myorg/frontend:latest
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "compose.db.yml"), []byte(`
services:
  redis:
    image: redis:7-alpine
`), 0644)

	// Create oops.yml at root
	os.WriteFile(filepath.Join(tmpDir, "oops.yml"), []byte(`
registries:
  gar: asia-southeast1-docker.pkg.dev/my-project/my-repo
  gh: ghcr.io/myorg
`), 0644)

	// 1. Resolve by registry alias shortcut: gar/api-service:v2.1.0
	aliasTargets, err := docker.ResolveTargets(tmpDir, []string{"gar/api-service:v2.1.0"})
	if err != nil {
		t.Fatalf("failed to resolve by alias gar/api-service:v2.1.0: %v", err)
	}
	if len(aliasTargets) != 2 {
		t.Fatalf("expected 2 targets (web-api, worker), got %d", len(aliasTargets))
	}

	// 2. Resolve by gh shortcut: gh/frontend:latest
	ghTargets, err := docker.ResolveTargets(tmpDir, []string{"gh/frontend:latest"})
	if err != nil {
		t.Fatalf("failed to resolve gh/frontend:latest: %v", err)
	}
	if len(ghTargets) != 1 || ghTargets[0].ServiceName != "frontend" {
		t.Errorf("expected frontend service, got %+v", ghTargets)
	}

	// 3. Resolve by image suffix: redis:7-alpine
	redisTargets, err := docker.ResolveTargets(tmpDir, []string{"redis:7-alpine"})
	if err != nil {
		t.Fatalf("failed to resolve redis:7-alpine: %v", err)
	}
	if len(redisTargets) != 1 || redisTargets[0].ServiceName != "redis" {
		t.Errorf("expected redis service, got %+v", redisTargets)
	}

	// 4. Resolve via ResolveTargetsByImage helper
	helperTargets, err := docker.ResolveTargetsByImage(tmpDir, "gar/api-service:v2.1.0")
	if err != nil {
		t.Fatalf("ResolveTargetsByImage failed: %v", err)
	}
	if len(helperTargets) != 2 {
		t.Fatalf("expected 2 targets via helper, got %d", len(helperTargets))
	}
}

func TestResolveTargetsWithExceptions(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yml"), []byte(`
services:
  caddy:
    image: caddy:alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  redis:
    image: redis:7-alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yml"), []byte(`
services:
  api:
    image: my-api:latest
  worker:
    image: my-worker:latest
`), 0644)

	// 1. Resolve all (which goes to .) with except /edge -> should return only db and apps
	allExceptEdge, err := docker.ResolveTargetsWithExceptions(tmpDir, nil, []string{"/edge"})
	if err != nil {
		t.Fatalf("ResolveTargetsWithExceptions failed: %v", err)
	}
	if len(allExceptEdge) != 2 {
		t.Fatalf("expected 2 targets (api, worker), got %d: %+v", len(allExceptEdge), allExceptEdge)
	}
	for _, target := range allExceptEdge {
		if target.StackName == "edge" {
			t.Errorf("expected target not to be in stack edge, got %s/%s", target.StackName, target.ServiceName)
		}
	}

	// 2. Resolve /db with except /db/mysql -> should return only redis
	dbExceptMysql, err := docker.ResolveTargetsWithExceptions(tmpDir, []string{"/db"}, []string{"/db/mysql"})
	if err != nil {
		t.Fatalf("ResolveTargetsWithExceptions /db except mysql failed: %v", err)
	}
	if len(dbExceptMysql) != 1 || dbExceptMysql[0].ServiceName != "redis" {
		t.Fatalf("expected only redis, got %+v", dbExceptMysql)
	}
}
