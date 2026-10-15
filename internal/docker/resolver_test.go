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
    image: mysql:8.0
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

	// Create oops.yaml at root
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
    image: mysql:8.0
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

func TestComposeGroupsAndServiceTemplatesMultiF(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Root compose.yaml with x-groups
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
services:
  gateway:
    image: caddy:latest

x-oops-groups:
  db:
    - mysql
    - redis
`), 0644)

	// 2. Service templates in services/
	servicesDir := filepath.Join(tmpDir, "services")
	_ = os.MkdirAll(servicesDir, 0755)

	_ = os.WriteFile(filepath.Join(servicesDir, "mysql.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4
`), 0644)

	_ = os.WriteFile(filepath.Join(servicesDir, "redis.yaml"), []byte(`
services:
  redis:
    image: redis:alpine
`), 0644)

	// 3. Local machine override: compose.local.yaml
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.local.yaml"), []byte(`
services:
  mysql:
    ports:
      - "3306:3306"
`), 0644)

	// Test 1: Resolve +db group
	groupTargets, err := docker.ResolveTargets(tmpDir, []string{"+db"})
	if err != nil {
		t.Fatalf("unexpected error resolving +db: %v", err)
	}
	if len(groupTargets) != 2 {
		t.Fatalf("expected 2 targets for +db, got %d", len(groupTargets))
	}

	// Verify tail-chaining in ComposeFiles:
	// 1: compose.yaml, 2: services/mysql.yaml, 3: compose.local.yaml
	for _, gt := range groupTargets {
		if len(gt.ComposeFiles) < 3 {
			t.Fatalf("expected at least 3 files in chain, got %+v", gt.ComposeFiles)
		}
		// First must be compose.yaml
		if filepath.Base(gt.ComposeFiles[0]) != "compose.yaml" {
			t.Errorf("expected first file compose.yaml, got %s", gt.ComposeFiles[0])
		}
		// Last must be compose.local.yaml
		lastFile := filepath.Base(gt.ComposeFiles[len(gt.ComposeFiles)-1])
		if lastFile != "compose.local.yaml" {
			t.Errorf("expected tail file compose.local.yaml, got %s", lastFile)
		}
	}

	// Test 2: Resolve single service mysql (auto-detects services/mysql.yaml)
	singleTargets, err := docker.ResolveTargets(tmpDir, []string{"mysql"})
	if err != nil {
		t.Fatalf("unexpected error resolving single service mysql: %v", err)
	}
	if len(singleTargets) != 1 || singleTargets[0].ServiceName != "mysql" {
		t.Fatalf("expected 1 mysql target, got %+v", singleTargets)
	}
	if filepath.Base(singleTargets[0].ComposeFiles[len(singleTargets[0].ComposeFiles)-1]) != "compose.local.yaml" {
		t.Errorf("expected tail local override in single target chain")
	}

	// Test 3: Legacy .yml and docker-compose.* are ignored
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.legacy.yml"), []byte(`
services:
  legacy:
    image: legacy:1.0
`), 0644)
	stacks, _, err := docker.DiscoverStacks(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverStacks failed: %v", err)
	}
	for _, s := range stacks {
		if s == "legacy" {
			t.Errorf("legacy .yml file should not be discovered as a stack")
		}
	}
}

func TestBuildComposeChainAndResolveTargetsWithOopsYaml(t *testing.T) {
	tmpDir := t.TempDir()

	// Root compose.yaml with base networks and app service
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
services:
  web:
    image: nginx:alpine
`), 0644)

	// oops.yaml with oops daemon service and x-oops config
	_ = os.WriteFile(filepath.Join(tmpDir, "oops.yaml"), []byte(`
services:
  oops:
    image: ghcr.io/noyzilla/oops:edge
x-oops:
  registries:
    gh: ghcr.io/myorg
`), 0644)

	// 1. Verify BuildComposeChain includes oops.yaml as second anchor
	chain := docker.BuildComposeChain(tmpDir, nil, ".")
	if len(chain) < 2 {
		t.Fatalf("expected at least 2 files in chain, got %v", chain)
	}
	if filepath.Base(chain[0]) != "compose.yaml" {
		t.Errorf("expected first file compose.yaml, got %s", chain[0])
	}
	if filepath.Base(chain[1]) != "oops.yaml" {
		t.Errorf("expected second file oops.yaml, got %s", chain[1])
	}

	// 2. Resolve target 'oops' service directly
	oopsTargets, err := docker.ResolveTargets(tmpDir, []string{"oops"})
	if err != nil {
		t.Fatalf("unexpected error resolving oops target: %v", err)
	}
	if len(oopsTargets) != 1 || oopsTargets[0].ServiceName != "oops" {
		t.Fatalf("expected 1 oops target, got %+v", oopsTargets)
	}
	if oopsTargets[0].StackName != "." {
		t.Errorf("expected oops target stack to be '.', got %q", oopsTargets[0].StackName)
	}

	// 3. Resolve all (nil targets) -> should include both 'web' and 'oops'
	allTargets, err := docker.ResolveTargets(tmpDir, nil)
	if err != nil {
		t.Fatalf("unexpected error resolving all targets: %v", err)
	}
	if len(allTargets) != 2 {
		t.Fatalf("expected 2 targets (web, oops), got %d: %+v", len(allTargets), allTargets)
	}
	names := []string{allTargets[0].ServiceName, allTargets[1].ServiceName}
	hasWeb := names[0] == "web" || names[1] == "web"
	hasOops := names[0] == "oops" || names[1] == "oops"
	if !hasWeb || !hasOops {
		t.Errorf("expected web and oops targets, got %v", names)
	}
}

func TestXOopsGroupsWithDirectServicesAndProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Root compose.yaml with x-oops-groups
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
x-oops-groups:
  lab:
    - caddy-proxy
    - mysql
    - /tools
`), 0644)

	// 2. Service templates in services/
	servicesDir := filepath.Join(tmpDir, "services")
	_ = os.MkdirAll(servicesDir, 0755)

	_ = os.WriteFile(filepath.Join(servicesDir, "caddy.yaml"), []byte(`
services:
  caddy-proxy:
    image: caddy:latest
`), 0644)

	_ = os.WriteFile(filepath.Join(servicesDir, "mysql.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4
`), 0644)

	// 3. Stack file compose.tools.yaml
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.tools.yaml"), []byte(`
services:
  httpbin:
    image: kennethreitz/httpbin
`), 0644)

	// Resolve +lab group
	targets, err := docker.ResolveTargets(tmpDir, []string{"+lab"})
	if err != nil {
		t.Fatalf("unexpected error resolving +lab: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets for +lab, got %d: %+v", len(targets), targets)
	}

	serviceMap := make(map[string]string)
	for _, tgt := range targets {
		serviceMap[tgt.ServiceName] = tgt.StackName
	}

	if serviceMap["caddy-proxy"] != "services/caddy" {
		t.Errorf("expected caddy-proxy in services/caddy, got %s", serviceMap["caddy-proxy"])
	}
	if serviceMap["mysql"] != "services/mysql" {
		t.Errorf("expected mysql in services/mysql, got %s", serviceMap["mysql"])
	}
	if serviceMap["httpbin"] != "tools" {
		t.Errorf("expected httpbin in stack tools, got %s", serviceMap["httpbin"])
	}
}

func TestXOopsDependsVsXOopsServices(t *testing.T) {
	tmpDir := t.TempDir()

	servicesDir := filepath.Join(tmpDir, "services")
	_ = os.MkdirAll(servicesDir, 0755)

	_ = os.WriteFile(filepath.Join(servicesDir, "mysql.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4
`), 0644)

	_ = os.WriteFile(filepath.Join(servicesDir, "redis.yaml"), []byte(`
services:
  redis:
    image: redis:alpine
`), 0644)

	// compose.app.yaml defines:
	// x-oops-depends: [mysql] (must auto-start mysql when /app is targeted)
	// x-oops-services: [redis] (compose chain includes redis.yaml, but redis is not auto-started)
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.app.yaml"), []byte(`
x-oops-depends:
  - mysql
x-oops-services:
  - redis
services:
  api:
    image: my-api:latest
`), 0644)

	targets, err := docker.ResolveTargets(tmpDir, []string{"/app"})
	if err != nil {
		t.Fatalf("unexpected error resolving /app: %v", err)
	}

	// Should resolve 2 targets to run: mysql (dependency) and api
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (mysql and api), got %d: %+v", len(targets), targets)
	}

	// First should be mysql dependency, second api
	if targets[0].ServiceName != "mysql" || targets[1].ServiceName != "api" {
		t.Errorf("unexpected target order or members: %+v", targets)
	}

	// Verify compose files for api include both mysql and redis in the chain
	apiTarget := targets[1]
	hasMysqlTmpl := false
	hasRedisTmpl := false
	for _, f := range apiTarget.ComposeFiles {
		if filepath.Base(f) == "mysql.yaml" {
			hasMysqlTmpl = true
		}
		if filepath.Base(f) == "redis.yaml" {
			hasRedisTmpl = true
		}
	}
	if !hasMysqlTmpl || !hasRedisTmpl {
		t.Errorf("expected api compose chain to include mysql.yaml and redis.yaml, got %v", apiTarget.ComposeFiles)
	}
}

func TestLocalPrecedenceAndCrossStackResolution(t *testing.T) {
	tmpDir := t.TempDir()

	servicesDir := filepath.Join(tmpDir, "services")
	_ = os.MkdirAll(servicesDir, 0755)

	// Global template services/mysql.yaml
	_ = os.WriteFile(filepath.Join(servicesDir, "mysql.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.0-template
`), 0644)

	// Stack 1: compose.crm.yaml with local mysql definition
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.crm.yaml"), []byte(`
services:
  mysql:
    image: mysql:8.4-crm-local
  crm-api:
    image: crm:latest
    depends_on:
      - mysql
`), 0644)

	// Stack 2: compose.tools2.yaml with redis
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.tools2.yaml"), []byte(`
services:
  redis:
    image: redis:alpine
  worker:
    image: worker:latest
`), 0644)

	// Test 1: oops up mysql (no slash) targets services/mysql template, NOT crm's local mysql
	tmplTargets, err := docker.ResolveTargets(tmpDir, []string{"mysql"})
	if err != nil {
		t.Fatalf("unexpected error resolving mysql: %v", err)
	}
	if len(tmplTargets) != 1 || tmplTargets[0].StackName != "services/mysql" {
		t.Errorf("expected services/mysql target, got %+v", tmplTargets)
	}

	// Test 2: oops up /crm/mysql targets crm's local mysql
	crmMysqlTargets, err := docker.ResolveTargets(tmpDir, []string{"/crm/mysql"})
	if err != nil {
		t.Fatalf("unexpected error resolving /crm/mysql: %v", err)
	}
	if len(crmMysqlTargets) != 1 || crmMysqlTargets[0].StackName != "crm" {
		t.Errorf("expected crm stack target, got %+v", crmMysqlTargets)
	}

	// Test 3: oops up /crm resolves crm-api and local mysql, NEVER pulling services/mysql.yaml into chain
	crmTargets, err := docker.ResolveTargets(tmpDir, []string{"/crm"})
	if err != nil {
		t.Fatalf("unexpected error resolving /crm: %v", err)
	}
	if len(crmTargets) != 2 {
		t.Fatalf("expected 2 targets for /crm, got %d", len(crmTargets))
	}
	for _, tgt := range crmTargets {
		if tgt.StackName != "crm" {
			t.Errorf("expected target to belong to crm stack, got %s", tgt.StackName)
		}
		for _, f := range tgt.ComposeFiles {
			if filepath.Base(f) == "mysql.yaml" {
				t.Errorf("local definition violated: services/mysql.yaml was pulled into compose chain: %v", tgt.ComposeFiles)
			}
		}
	}

	// Test 4: Root compose.yaml with group referencing /tools2/redis
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte(`
x-oops-groups:
  lab:
    - mysql
    - /tools2/redis
`), 0644)

	groupTargets, err := docker.ResolveTargets(tmpDir, []string{"+lab"})
	if err != nil {
		t.Fatalf("unexpected error resolving +lab: %v", err)
	}
	if len(groupTargets) != 2 {
		t.Fatalf("expected 2 targets for +lab, got %d: %+v", len(groupTargets), groupTargets)
	}

	// First should be services/mysql, second tools2/redis (not worker!)
	serviceMap := make(map[string]string)
	for _, tgt := range groupTargets {
		serviceMap[tgt.ServiceName] = tgt.StackName
	}
	if serviceMap["mysql"] != "services/mysql" {
		t.Errorf("expected mysql in services/mysql, got %s", serviceMap["mysql"])
	}
	if serviceMap["redis"] != "tools2" {
		t.Errorf("expected redis in tools2, got %s", serviceMap["redis"])
	}
	if _, hasWorker := serviceMap["worker"]; hasWorker {
		t.Errorf("worker should not be targeted when /tools2/redis was specified in group")
	}
}
