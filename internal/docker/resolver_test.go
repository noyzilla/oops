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

	// Create stacks: edge, db, apps under stacks/
	stacksDir := filepath.Join(tmpDir, "stacks")
	edgeDir := filepath.Join(stacksDir, "edge")
	dbDir := filepath.Join(stacksDir, "db")
	appsDir := filepath.Join(stacksDir, "apps")

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
	if all[0].StackName != "edge" || all[1].StackName != "db" || all[3].StackName != "apps" {
		t.Errorf("unexpected stack names for targets: %+v", all)
	}

	// 2. Resolve stack target /apps
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

func TestResolveGroupsAndDefaultGroup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-groups-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stacksDir := filepath.Join(tmpDir, "stacks")
	edgeDir := filepath.Join(stacksDir, "edge")
	dbDir := filepath.Join(stacksDir, "db")
	utilsDir := filepath.Join(stacksDir, "utils")
	appsDir := filepath.Join(stacksDir, "apps")

	os.MkdirAll(edgeDir, 0755)
	os.MkdirAll(dbDir, 0755)
	os.MkdirAll(utilsDir, 0755)
	os.MkdirAll(appsDir, 0755)

	os.WriteFile(filepath.Join(edgeDir, "compose.yml"), []byte(`
services:
  caddy:
    image: caddy:latest
`), 0644)

	os.WriteFile(filepath.Join(dbDir, "compose.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  postgres:
    image: postgres:16
  redis:
    image: redis:alpine
`), 0644)

	os.WriteFile(filepath.Join(utilsDir, "compose.yml"), []byte(`
services:
  oops:
    image: oops:latest
`), 0644)

	os.WriteFile(filepath.Join(appsDir, "compose.yml"), []byte(`
services:
  web:
    image: web:latest
`), 0644)

	// Create stacks/groups.yml
	os.WriteFile(filepath.Join(stacksDir, "groups.yml"), []byte(`
groups:
  core:
    - /edge
    - mysql
    - redis
    - /utils
  pg:
    - /edge
    - postgres
    - redis
    - /utils
  minimal:
    - /edge
`), 0644)

	// 1. Resolve explicit @core group
	coreTargets, err := docker.ResolveTargets(tmpDir, []string{"@core"})
	if err != nil {
		t.Fatalf("failed to resolve @core: %v", err)
	}
	if len(coreTargets) != 4 {
		t.Fatalf("expected 4 targets for @core, got %d", len(coreTargets))
	}
	// Verify postgres & web are NOT in core
	for _, tgt := range coreTargets {
		if tgt.ServiceName == "postgres" || tgt.ServiceName == "web" {
			t.Errorf("unexpected service %s in core group", tgt.ServiceName)
		}
	}

	// 2. Resolve explicit @pg group
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
		t.Errorf("expected pg group to contain postgres and not mysql, got pgTargets: %+v", pgTargets)
	}

	// 3. Test OOPS_DEFAULT_GROUP in .env
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("OOPS_DEFAULT_GROUP=minimal\n"), 0644)
	defTargets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to resolve targets with OOPS_DEFAULT_GROUP=minimal in .env: %v", err)
	}
	if len(defTargets) != 1 || defTargets[0].ServiceName != "caddy" {
		t.Errorf("expected 1 caddy target for minimal group, got: %+v", defTargets)
	}

	// 4. Test OOPS_DEFAULT_GROUP env override
	t.Setenv("OOPS_DEFAULT_GROUP", "pg")
	envTargets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to resolve targets with OOPS_DEFAULT_GROUP=pg: %v", err)
	}
	if len(envTargets) != 4 {
		t.Fatalf("expected 4 targets for OOPS_DEFAULT_GROUP=pg, got %d", len(envTargets))
	}

	// 5. Test unknown group error
	_, err = docker.ResolveTargets(tmpDir, []string{"@unknown"})
	if err == nil {
		t.Errorf("expected error for non-existent group @unknown, got nil")
	}
}

func TestResolveImageAliasesAndRegistryShortcuts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-img-alias-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stacksDir := filepath.Join(tmpDir, "stacks")
	appsDir := filepath.Join(stacksDir, "apps")
	dbDir := filepath.Join(stacksDir, "db")

	os.MkdirAll(appsDir, 0755)
	os.MkdirAll(dbDir, 0755)

	os.WriteFile(filepath.Join(appsDir, "compose.yml"), []byte(`
services:
  web-api:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  worker:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  frontend:
    image: ghcr.io/myorg/frontend:latest
`), 0644)

	os.WriteFile(filepath.Join(dbDir, "compose.yml"), []byte(`
services:
  redis:
    image: redis:7-alpine
`), 0644)

	// Create stacks/oops.yml
	os.WriteFile(filepath.Join(stacksDir, "oops.yml"), []byte(`
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
	stacksDir := filepath.Join(tmpDir, "stacks")
	_ = os.MkdirAll(filepath.Join(stacksDir, "edge"), 0755)
	_ = os.MkdirAll(filepath.Join(stacksDir, "db"), 0755)
	_ = os.MkdirAll(filepath.Join(stacksDir, "apps"), 0755)

	_ = os.WriteFile(filepath.Join(stacksDir, "edge", "compose.yml"), []byte(`
services:
  caddy:
    image: caddy:alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(stacksDir, "db", "compose.yml"), []byte(`
services:
  mysql:
    image: mysql:8.0
  redis:
    image: redis:7-alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(stacksDir, "apps", "compose.yml"), []byte(`
services:
  api:
    image: my-api:latest
  worker:
    image: my-worker:latest
`), 0644)

	_ = os.WriteFile(filepath.Join(stacksDir, "oops.yml"), []byte(`
groups:
  core:
    - /edge
    - mysql
    - redis
  apps:
    - /apps
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
		if target.StackName != "apps" {
			t.Errorf("expected target to be in stack apps, got %s/%s", target.StackName, target.ServiceName)
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

