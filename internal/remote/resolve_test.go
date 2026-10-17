package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRequiredDependencies(t *testing.T) {
	tempDir := t.TempDir()

	// Create compose.crm.yaml with x-oops-services and depends_on
	crmContent := `x-oops-services:
  - postgres
  - redis
services:
  crm-app:
    image: custom/crm:latest
    depends_on:
      - caddy
`
	_ = os.WriteFile(filepath.Join(tempDir, "compose.crm.yaml"), []byte(crmContent), 0644)

	// Create dummy service templates
	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "postgres.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "redis.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "caddy.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "unused.yaml"), []byte("services: {}\n"), 0644)

	// Create capsule dir with overrides
	capsuleDir := filepath.Join(tempDir, "@prod")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte("name: oops\n"), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "crm", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := inspector.Services["postgres"]; !ok {
		t.Errorf("expected postgres in resolved services")
	}
	if _, ok := inspector.Services["redis"]; !ok {
		t.Errorf("expected redis in resolved services")
	}
	if _, ok := inspector.Services["caddy"]; !ok {
		t.Errorf("expected caddy in resolved services")
	}
	if _, ok := inspector.Services["unused"]; ok {
		t.Errorf("unused service should not be in resolved services")
	}

	if len(inspector.ServiceFiles) != 3 {
		t.Errorf("expected 3 service files, got %d: %v", len(inspector.ServiceFiles), inspector.ServiceFiles)
	}
}

func TestResolveRequiredDependencies_RemoteLocalPrecedence(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Stack compose.legacy.yaml defines local mysql (5.7) and depends on mysql & redis
	legacyContent := `services:
  mysql:
    image: mysql:5.7
  app:
    image: custom/legacy:latest
    depends_on:
      - mysql
      - redis
`
	_ = os.WriteFile(filepath.Join(tempDir, "compose.legacy.yaml"), []byte(legacyContent), 0644)

	// 2. Workspace root service templates: mysql (8.0), redis
	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "mysql.yaml"), []byte("services: { mysql: { image: 'mysql:8.4' } }\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "redis.yaml"), []byte("services: { redis: {} }\n"), 0644)

	// 3. Capsule @prod defines custom redis in capsule services/
	capsuleDir := filepath.Join(tempDir, "@prod")
	_ = os.MkdirAll(filepath.Join(capsuleDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "services", "redis.yaml"), []byte("services: { redis: { image: 'redis:prod' } }\n"), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "legacy", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// mysql is defined in compose.legacy.yaml -> services/mysql.yaml must NOT be included in ServiceFiles
	// redis is defined in @prod/services/redis.yaml -> services/redis.yaml must NOT be included in ServiceFiles
	if len(inspector.ServiceFiles) != 0 {
		t.Errorf("expected 0 ServiceFiles because both mysql and redis are locally/capsule defined, got: %v", inspector.ServiceFiles)
	}
}

// Case 1: compose.app.yaml calls mysql, @remote/compose.yaml defines mysql -> services/mysql.yaml must NOT be sent
func TestResolveRequiredDependencies_Case1_RemoteCapsuleService(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "compose.app.yaml"), []byte(`services:
  app:
    image: app:latest
    depends_on:
      - mysql
`), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "mysql.yaml"), []byte("services: { mysql: {} }\n"), 0644)

	capsuleDir := filepath.Join(tempDir, "@remote")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte(`services:
  mysql:
    image: mysql:8.4
`), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "app", "remote")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(inspector.ServiceFiles) != 0 {
		t.Errorf("Case 1 failed: expected 0 ServiceFiles, got %v", inspector.ServiceFiles)
	}
}

// Case 2: compose.db.yaml has mysql, compose.app.yaml calls /db/mysql -> do not send services/mysql, but send compose.db.yaml
func TestResolveRequiredDependencies_Case2_CrossStackDependency(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "compose.db.yaml"), []byte(`services:
  mysql:
    image: mysql:8.4
`), 0644)

	_ = os.WriteFile(filepath.Join(tempDir, "compose.app.yaml"), []byte(`x-oops-services:
  - /db/mysql
services:
  app:
    image: app:latest
`), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "mysql.yaml"), []byte("services: { mysql: {} }\n"), 0644)

	capsuleDir := filepath.Join(tempDir, "@remote")
	_ = os.MkdirAll(capsuleDir, 0755)

	// 1. Pushing only "/app" must error because /db is not in push targets and not in capsule
	_, err := ResolveRequiredDependencies(tempDir, "app", "remote")
	if err == nil {
		t.Fatalf("Case 2A failed: expected error when pushing /app without /db, got nil")
	}
	expectedErr := "stack '/app' depends on '/db/mysql', but stack '/db' is not in push targets and does not exist in capsule '/@remote'"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("Case 2A failed: unexpected error message: %v", err)
	}

	// 2. Pushing "/app,/db" explicitly must succeed: sends compose.app.yaml & compose.db.yaml, and suppresses services/mysql
	inspector, err := ResolveRequiredDependencies(tempDir, "/app,/db", "remote")
	if err != nil {
		t.Fatalf("Case 2B failed: unexpected error for /app,/db: %v", err)
	}

	// services/mysql.yaml must NOT be included
	if len(inspector.ServiceFiles) != 0 {
		t.Errorf("Case 2B failed: expected 0 ServiceFiles, got %v", inspector.ServiceFiles)
	}

	// compose.db.yaml and compose.app.yaml MUST be included in StackFiles
	foundDB, foundApp := false, false
	for _, sf := range inspector.StackFiles {
		if sf == "compose.db.yaml" {
			foundDB = true
		}
		if sf == "compose.app.yaml" {
			foundApp = true
		}
	}
	if !foundDB || !foundApp {
		t.Errorf("Case 2B failed: expected both compose.app.yaml and compose.db.yaml in StackFiles, got: %v", inspector.StackFiles)
	}
}

func TestParseStackTargets(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"app", []string{"app"}},
		{"/app", []string{"app"}},
		{"/app,/db", []string{"app", "db"}},
		{"app, db", []string{"app", "db"}},
		{"/app, /db, /cache", []string{"app", "db", "cache"}},
		{"/app,/app", []string{"app"}},
	}
	for _, tt := range tests {
		got := ParseStackTargets(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ParseStackTargets(%q) length mismatch: got %v, expected %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("ParseStackTargets(%q)[%d] = %q, expected %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

// Case 3: In @remote there is compose.db.yaml -> take compose.db.yaml from remote capsule, NOT root /compose.db.yaml
func TestResolveRequiredDependencies_Case3_RemoteCapsuleStackOverride(t *testing.T) {
	tempDir := t.TempDir()

	// Root workspace compose.db.yaml (dev version)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.db.yaml"), []byte(`services:
  mysql:
    image: mysql:dev
`), 0644)

	_ = os.WriteFile(filepath.Join(tempDir, "compose.app.yaml"), []byte(`x-oops-services:
  - /db/mysql
services:
  app:
    image: app:latest
`), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "mysql.yaml"), []byte("services: { mysql: {} }\n"), 0644)

	// Remote capsule has its own compose.db.yaml (prod version)
	capsuleDir := filepath.Join(tempDir, "@remote")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.db.yaml"), []byte(`services:
  mysql:
    image: mysql:prod
`), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "app", "remote")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// services/mysql.yaml must NOT be sent
	if len(inspector.ServiceFiles) != 0 {
		t.Errorf("Case 3 failed: expected 0 ServiceFiles, got %v", inspector.ServiceFiles)
	}

	// compose.db.yaml from root workspace MUST NOT be in StackFiles
	for _, sf := range inspector.StackFiles {
		if sf == "compose.db.yaml" {
			t.Errorf("Case 3 failed: root compose.db.yaml must NOT be in StackFiles when overridden by capsule")
		}
	}

	// compose.db.yaml MUST be in CapsuleStackFiles
	foundCapsuleDB := false
	for _, csf := range inspector.CapsuleStackFiles {
		if csf == "compose.db.yaml" {
			foundCapsuleDB = true
		}
	}
	if !foundCapsuleDB {
		t.Errorf("Case 3 failed: compose.db.yaml must be in CapsuleStackFiles, got: %v", inspector.CapsuleStackFiles)
	}
}

func TestResolveRequiredDependencies_XOopsStackAndServiceAlias(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "compose.edge.yaml"), []byte("services:\n  caddy-proxy:\n    image: lucaslorentz/caddy-docker-proxy:alpine\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.db.yaml"), []byte("services:\n  mysql:\n    image: mysql:8.4\n  postgres:\n    image: postgres:16-alpine\n  redis:\n    image: redis:7-alpine\n"), 0644)

	capsuleDir := filepath.Join(tempDir, "@b9tpe")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte(`x-oops-stack:
  - /edge
  - /db
`), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "", "b9tpe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := map[string]bool{
		"compose.edge.yaml": false,
		"compose.db.yaml":   false,
	}
	for _, sf := range inspector.StackFiles {
		if _, ok := expectedFiles[sf]; ok {
			expectedFiles[sf] = true
		}
	}
	for file, found := range expectedFiles {
		if !found {
			t.Errorf("expected %s in StackFiles, got: %v", file, inspector.StackFiles)
		}
	}
}

func TestBuildLocalTarArgs(t *testing.T) {
	darwinArgs := buildLocalTarArgs("darwin", "-czf", "-", "-C", ".", "compose.yaml")
	if len(darwinArgs) < 2 || darwinArgs[0] != "--no-xattrs" || darwinArgs[1] != "--no-mac-metadata" {
		t.Errorf("expected --no-xattrs and --no-mac-metadata on darwin, got: %v", darwinArgs)
	}

	linuxArgs := buildLocalTarArgs("linux", "-czf", "-", "-C", ".", "compose.yaml")
	if len(linuxArgs) == 0 || linuxArgs[0] != "-czf" {
		t.Errorf("expected no darwin flags on linux, got: %v", linuxArgs)
	}
}

func TestResolveRequiredDependencies_OmittedStackCapsuleScope(t *testing.T) {
	tempDir := t.TempDir()

	// Root workspace stacks (crm and billing) that do NOT exist in capsule
	_ = os.WriteFile(filepath.Join(tempDir, "compose.crm.yaml"), []byte("services:\n  crm:\n    image: crm:latest\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.billing.yaml"), []byte("services:\n  billing:\n    image: billing:latest\n"), 0644)

	// Capsule has compose.edge.yaml
	capsuleDir := filepath.Join(tempDir, "@remote")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.edge.yaml"), []byte("services:\n  caddy:\n    image: caddy:latest\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte("name: oops\n"), 0644)

	// Resolve with stack == ""
	inspector, err := ResolveRequiredDependencies(tempDir, "", "remote")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Root stacks compose.crm.yaml and compose.billing.yaml must NOT be included in StackFiles!
	for _, sf := range inspector.StackFiles {
		if sf == "compose.crm.yaml" || sf == "compose.billing.yaml" {
			t.Errorf("omitted stack must not include root workspace stack %s", sf)
		}
	}

	// Capsule stack compose.edge.yaml MUST be in CapsuleStackFiles
	foundEdge := false
	for _, csf := range inspector.CapsuleStackFiles {
		if csf == "compose.edge.yaml" {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Errorf("expected compose.edge.yaml in CapsuleStackFiles, got: %v", inspector.CapsuleStackFiles)
	}
}

func TestResolveRequiredDependencies_XOopsStackAndDependsOnWarning(t *testing.T) {
	tempDir := t.TempDir()

	// Root workspace stacks: compose.edge.yaml, compose.db.yaml, compose.app.yaml, compose.unused.yaml
	_ = os.WriteFile(filepath.Join(tempDir, "compose.edge.yaml"), []byte("services:\n  caddy-proxy:\n    image: caddy:latest\n  oops:\n    image: oops:latest\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.db.yaml"), []byte("services:\n  mysql:\n    image: mysql:8.4\n  redis:\n    image: redis:7\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.app.yaml"), []byte(`services:
  app1:
    image: app1:latest
    depends_on:
      - mysql
`), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "compose.unused.yaml"), []byte("services:\n  unused:\n    image: unused:latest\n"), 0644)

	// Capsule compose.yaml declares x-oops-stack with /edge and /app, but omits /db!
	capsuleDir := filepath.Join(tempDir, "@prod")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte(`x-oops-stack:
  - /edge
  - /app
`), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include compose.edge.yaml, compose.app.yaml, and transitive compose.db.yaml, but NOT compose.unused.yaml
	for _, sf := range inspector.StackFiles {
		if sf == "compose.unused.yaml" {
			t.Errorf("compose.unused.yaml should not be included in StackFiles")
		}
	}

	// Because app1 has depends_on: [mysql] and /db (or /db/mysql) is NOT in x-oops-stack, a warning MUST be generated!
	if len(inspector.Warnings) == 0 {
		t.Fatalf("expected pre-flight warning for undeclared depends_on 'mysql' in x-oops-stack, got 0 warnings")
	}
	foundWarn := false
	for _, w := range inspector.Warnings {
		if strings.Contains(w, "mysql") && strings.Contains(w, "x-oops-stack") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning mentioning mysql and x-oops-stack, got: %v", inspector.Warnings)
	}
}
