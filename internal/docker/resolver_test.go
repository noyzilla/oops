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
	tmpDir := t.TempDir()

	// Create flat stacks at root strictly using .yaml
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yaml"), []byte(`
services:
  caddy:
    image: caddy:latest
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4
  redis:
    image: redis:alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
services:
  app-web:
    image: myapp:web
  app-worker:
    image: myapp:worker
`), 0644)

	// 1. Resolve all (empty targets) -> targets root compose.yaml (.)
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
	if len(wildcardTargets) != 2 {
		t.Fatalf("expected 2 targets for app.., got %d", len(wildcardTargets))
	}
}

func TestResolveImageAliasesAndRegistryShortcuts(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.apps.yaml"), []byte(`
services:
  web-api:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  worker:
    image: asia-southeast1-docker.pkg.dev/my-project/my-repo/api-service:v2.1.0
  frontend:
    image: ghcr.io/myorg/frontend:latest
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`
services:
  redis:
    image: redis:7-alpine
`), 0644)

	// Create oops.yaml at root (pure config)
	_ = os.WriteFile(filepath.Join(tmpDir, "oops.yaml"), []byte(`
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
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yaml"), []byte(`
services:
  caddy:
    image: caddy:alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4
  redis:
    image: redis:7-alpine
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
services:
  api:
    image: my-api:latest
  worker:
    image: my-worker:latest
`), 0644)

	// 1. Resolve all (which goes to .) with except /edge -> should return only api and worker
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

func TestXOopsStackOrderingAndSelectiveAutoStart(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Root compose.yaml with x-oops-stack specifying order: /edge, then /db/redis, then /db/mysql
	// Notice /tools is NOT in x-oops-stack, and /db/postgres is NOT in x-oops-stack!
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
x-oops-stack:
  - /edge
  - /db/redis
  - /db/mysql
networks:
  default:
    name: oops_default
`), 0644)

	// 2. Stack compose.edge.yaml with its own x-oops-stack order: caddy-proxy, then oops
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yaml"), []byte(`
x-oops-stack:
  - caddy-proxy
  - oops
services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
  caddy-proxy:
    image: caddy:latest
`), 0644)

	// 3. Stack compose.db.yaml with mysql, postgres, redis
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`
x-oops-stack:
  - mysql
  - redis
services:
  mysql:
    image: mysql:8.4
  postgres:
    image: postgres:16-alpine
  redis:
    image: redis:7-alpine
`), 0644)

	// 4. Stack compose.tools.yaml (unlisted in root compose.yaml)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.tools.yaml"), []byte(`
services:
  mailpit:
    image: axllent/mailpit:latest
`), 0644)

	// Test A: Default 'oops up' (nil targets) must strictly follow compose.yaml x-oops-stack order
	// Expected: caddy-proxy, oops, redis, mysql (4 services, NO postgres, NO mailpit)
	targets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error resolving default targets: %v", err)
	}
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets from x-oops-stack, got %d: %+v", len(targets), targets)
	}
	expectedOrder := []string{"caddy-proxy", "oops", "redis", "mysql"}
	for i, exp := range expectedOrder {
		if targets[i].ServiceName != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, targets[i].ServiceName)
		}
	}

	// Test B: Targeting '/db' uses compose.db.yaml's x-oops-stack (mysql, redis -> omits postgres)
	dbTargets, err := docker.ResolveTargets(tmpDir, []string{"/db"})
	if err != nil {
		t.Fatalf("unexpected error resolving /db: %v", err)
	}
	if len(dbTargets) != 2 {
		t.Fatalf("expected 2 targets for /db (mysql, redis), got %d: %+v", len(dbTargets), dbTargets)
	}
	if dbTargets[0].ServiceName != "mysql" || dbTargets[1].ServiceName != "redis" {
		t.Errorf("expected [mysql, redis], got [%s, %s]", dbTargets[0].ServiceName, dbTargets[1].ServiceName)
	}

	// Test C: Explicitly targeting '/db/postgres' starts postgres even though it is omitted from x-oops-stack
	pgTargets, err := docker.ResolveTargets(tmpDir, []string{"/db/postgres"})
	if err != nil {
		t.Fatalf("unexpected error resolving /db/postgres: %v", err)
	}
	if len(pgTargets) != 1 || pgTargets[0].ServiceName != "postgres" {
		t.Fatalf("expected postgres target, got %+v", pgTargets)
	}
}

func TestBuildComposeChainPureConfigOopsYaml(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
networks:
  default:
    name: oops_default
`), 0644)

	// Pure config oops.yaml must NOT be added to docker compose -f chain
	_ = os.WriteFile(filepath.Join(tmpDir, "oops.yaml"), []byte(`
x-oops:
  secret: test
`), 0644)

	_ = os.WriteFile(filepath.Join(tmpDir, "compose.edge.yaml"), []byte(`
services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
`), 0644)

	chain := docker.BuildComposeChain(tmpDir, []string{filepath.Join(tmpDir, "compose.edge.yaml")}, "edge")
	for _, f := range chain {
		if filepath.Base(f) == "oops.yaml" {
			t.Errorf("pure config oops.yaml should not be included in compose -f chain: %v", chain)
		}
	}
}

func TestLocalServiceVersionDiscrepancyAndPrecedence(t *testing.T) {
	tmpDir := t.TempDir()

	// Shared stack: compose.db.yaml (MySQL 8.0)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.0
    container_name: mysql
`), 0644)

	// Legacy project: compose.legacy.yaml using local MySQL 5.7
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.legacy.yaml"), []byte(`
services:
  mysql:
    image: mysql:5.7
    container_name: legacy_mysql
  app:
    image: legacy-app:v1
    depends_on:
      - mysql
`), 0644)

	// 1. Resolve /legacy: must use local mysql 5.7, never pull compose.db.yaml (8.0)
	legacyTargets, err := docker.ResolveTargets(tmpDir, []string{"/legacy"})
	if err != nil {
		t.Fatalf("failed to resolve /legacy: %v", err)
	}
	if len(legacyTargets) != 2 {
		t.Fatalf("expected 2 targets for /legacy, got %d", len(legacyTargets))
	}

	for _, tgt := range legacyTargets {
		if tgt.StackName != "legacy" {
			t.Errorf("expected target to belong to legacy stack, got %s", tgt.StackName)
		}
		if tgt.ServiceName == "mysql" && tgt.Image != "mysql:5.7" {
			t.Errorf("expected mysql:5.7 in legacy stack, got %s", tgt.Image)
		}
		for _, f := range tgt.ComposeFiles {
			if filepath.Base(f) == "compose.db.yaml" {
				t.Errorf("compose.db.yaml (8.0) was illegally pulled into compose chain for legacy stack: %v", tgt.ComposeFiles)
			}
		}
	}

	// 2. Resolve /legacy/mysql: specifically targets local mysql 5.7
	specificTargets, err := docker.ResolveTargets(tmpDir, []string{"/legacy/mysql"})
	if err != nil {
		t.Fatalf("failed to resolve /legacy/mysql: %v", err)
	}
	if len(specificTargets) != 1 || specificTargets[0].StackName != "legacy" || specificTargets[0].Image != "mysql:5.7" {
		t.Errorf("expected legacy mysql 5.7, got %+v", specificTargets)
	}
}
