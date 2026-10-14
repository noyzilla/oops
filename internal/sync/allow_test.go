package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllowRuleSet_Defaults(t *testing.T) {
	rules, err := LoadAllowRuleSet(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error loading defaults: %v", err)
	}

	allowed := []string{
		".env",
		"compose.yaml",
		"oops.yaml",
		"compose.local.yaml",
		"compose.crm.local.yaml",
		"config/caddy/Caddyfile",
		"config/env/crm.env",
		"config/secrets/db_pass.txt",
	}

	for _, p := range allowed {
		if !rules.IsAllowed(p) {
			t.Errorf("expected %q to be allowed, but was rejected", p)
		}
	}

	denied := []string{
		"remote.yaml",
		".oopsallow",
		"compose.yaml.example",
		"data/mysql/ibdata1",
		"data/redis/dump.rdb",
		"backups/2026-10-14.tar.gz",
		"debug.log",
		"docker.sock",
		".git/config",
		".scratch/plan.md",
		"random.txt",
	}

	for _, p := range denied {
		if rules.IsAllowed(p) {
			t.Errorf("expected %q to be denied, but was allowed", p)
		}
	}
}

func TestAllowRuleSet_CustomFile(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), ".oopsallow")
	content := `# Custom allow rules
/.env
/custom/**
`
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	rules, err := LoadAllowRuleSet(tempFile)
	if err != nil {
		t.Fatalf("failed loading custom rule set: %v", err)
	}

	if !rules.IsAllowed(".env") {
		t.Errorf("expected .env to be allowed")
	}
	if !rules.IsAllowed("custom/sub/file.txt") {
		t.Errorf("expected custom/sub/file.txt to be allowed")
	}
	if rules.IsAllowed("compose.yaml") {
		t.Errorf("expected compose.yaml to be denied with custom rules")
	}
}
