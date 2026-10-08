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
	if all[0].StackName != "edge" || all[1].StackName != "db" || all[3].StackName != "." {
		t.Errorf("unexpected stack names for targets: %+v", all)
	}

	// 2. Resolve stack target /.
	apps, err := docker.ResolveTargets(tmpDir, []string{"/."})
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

func TestResolveProfilesAndDefaultProfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-profiles-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "compose.edge.yml"), []byte(`
services:
  caddy:
    image: caddy:latest
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "compose.db.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  postgres:
    image: postgres:16
  redis:
    image: redis:alpine
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "compose.utils.yml"), []byte(`
services:
  oops:
    image: oops:latest
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "compose.apps.yml"), []byte(`
services:
  web:
    image: web:latest
`), 0644)

	// Create oops.yml with profiles
	os.WriteFile(filepath.Join(tmpDir, "oops.yml"), []byte(`
profiles:
  lab:
    - /edge
    - mysql
    - redis
    - /utils
  pg:
    - /edge
    - postgres
    - redis
    - /utils
  default:
    - /edge
`), 0644)

	// 1. Resolve explicit @lab profile
	labTargets, err := docker.ResolveTargets(tmpDir, []string{"@lab"})
	if err != nil {
		t.Fatalf("failed to resolve @lab: %v", err)
	}
	if len(labTargets) != 4 {
		t.Fatalf("expected 4 targets for @lab, got %d", len(labTargets))
	}
	// Verify postgres & web are NOT in lab
	for _, tgt := range labTargets {
		if tgt.ServiceName == "postgres" || tgt.ServiceName == "web" {
			t.Errorf("unexpected service %s in lab profile", tgt.ServiceName)
		}
	}

	// 2. Resolve explicit @pg profile
	pgTargets, err := docker.ResolveTargets(tmpDir, []string{"@pg"})
	if err != nil {
		t.Fatalf("failed to resolve @pg: %v", err)
	}
	if len(pgTargets) != 4 {
		t.Fatalf("expected 4 targets for @pg, got %d", len(pgTargets))
	}
	hasPostgres := false
	hasMysql := false
	for _, tgt := range pgTargets {
		if tgt.ServiceName == "postgres" {
			hasPostgres = true
		}
		if tgt.ServiceName == "mysql" {
			hasMysql = true
		}
	}
	if !hasPostgres || hasMysql {
		t.Errorf("expected pg profile to contain postgres and not mysql, got pgTargets: %+v", pgTargets)
	}

	// 3. Test default profile when no targets passed (reads default from oops.yml)
	defTargets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to resolve targets with default profile in oops.yml: %v", err)
	}
	if len(defTargets) != 1 || defTargets[0].ServiceName != "caddy" {
		t.Errorf("expected 1 caddy target for default profile, got: %+v", defTargets)
	}

	// 4. Test Dynamic @all (not explicitly declared in profiles)
	allTargets, err := docker.ResolveTargets(tmpDir, []string{"@all"})
	if err != nil {
		t.Fatalf("failed to resolve dynamic @all: %v", err)
	}
	if len(allTargets) != 6 { // caddy, mysql, postgres, redis, oops, web
		t.Fatalf("expected 6 targets for dynamic @all, got %d", len(allTargets))
	}

	// 5. Test unknown profile error
	_, err = docker.ResolveTargets(tmpDir, []string{"@unknown"})
	if err == nil {
		t.Errorf("expected error for non-existent profile @unknown, got nil")
	}
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
groups:
  apps:
    - /apps
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

	_ = os.WriteFile(filepath.Join(tmpDir, "oops.yml"), []byte(`
profiles:
  core:
    - /edge
    - mysql
    - redis
  apps:
    - /.
`), 0644)

	// 1. Resolve all with except @core -> should return only apps (api, worker)
	allExceptCore, err := docker.ResolveTargetsWithExceptions(tmpDir, nil, []string{"@core"})
	if err != nil {
		t.Fatalf("ResolveTargetsWithExceptions failed: %v", err)
	}
	if len(allExceptCore) != 2 {
		t.Fatalf("expected 2 targets (api, worker), got %d: %+v", len(allExceptCore), allExceptCore)
	}
	for _, target := range allExceptCore {
		if target.StackName != "." {
			t.Errorf("expected target to be in stack ., got %s/%s", target.StackName, target.ServiceName)
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

