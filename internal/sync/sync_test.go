package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectSyncFiles(t *testing.T) {
	tempDir := t.TempDir()
	capsuleDir := filepath.Join(tempDir, "@prod")
	if err := os.MkdirAll(filepath.Join(capsuleDir, "config", "env"), 0755); err != nil {
		t.Fatalf("failed creating env dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(capsuleDir, "config", "secrets"), 0755); err != nil {
		t.Fatalf("failed creating secrets dir: %v", err)
	}

	// Create test files
	_ = os.WriteFile(filepath.Join(capsuleDir, "remote.yaml"), []byte("host: test\npath: ~/box\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, ".env"), []byte("APP_ENV=production\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "config", "env", "db.env"), []byte("DB_HOST=127.0.0.1\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "config", "secrets", "api_key.txt"), []byte("secret123\n"), 0644)

	// 1. Test SyncAll
	filesAll, err := CollectSyncFiles(capsuleDir, Options{SyncAll: true})
	if err != nil {
		t.Fatalf("CollectSyncFiles failed: %v", err)
	}
	// remote.yaml must NOT be included!
	for _, f := range filesAll {
		if f == "remote.yaml" {
			t.Errorf("remote.yaml must never be included in sync files")
		}
	}
	if len(filesAll) != 3 {
		t.Fatalf("expected 3 files (.env, config/env/db.env, config/secrets/api_key.txt), got %d: %v", len(filesAll), filesAll)
	}

	// 2. Test SyncEnv only
	filesEnv, err := CollectSyncFiles(capsuleDir, Options{SyncEnv: true})
	if err != nil {
		t.Fatalf("CollectSyncFiles failed: %v", err)
	}
	if len(filesEnv) != 2 {
		t.Fatalf("expected 2 env files, got %d: %v", len(filesEnv), filesEnv)
	}

	// 3. Test SyncSecret only
	filesSecret, err := CollectSyncFiles(capsuleDir, Options{SyncSecret: true})
	if err != nil {
		t.Fatalf("CollectSyncFiles failed: %v", err)
	}
	if len(filesSecret) != 1 || filesSecret[0] != filepath.Join("config", "secrets", "api_key.txt") {
		t.Fatalf("expected 1 secret file, got: %v", filesSecret)
	}
}

func TestCollectCapsuleFiles_MultiStackFilter(t *testing.T) {
	tempDir := t.TempDir()
	capsuleDir := filepath.Join(tempDir, "@prod")
	_ = os.MkdirAll(capsuleDir, 0755)

	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.app.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.db.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.cache.yaml"), []byte("services: {}\n"), 0644)

	rules, err := LoadAllowRuleSet(filepath.Join(capsuleDir, ".oopsallow"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Filter for "/app,/db"
	files, err := CollectCapsuleFiles(capsuleDir, rules, "/app,/db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundApp, foundDB, foundCache := false, false, false
	for _, f := range files {
		if f == "compose.app.yaml" {
			foundApp = true
		}
		if f == "compose.db.yaml" {
			foundDB = true
		}
		if f == "compose.cache.yaml" {
			foundCache = true
		}
	}

	if !foundApp || !foundDB {
		t.Errorf("expected compose.app.yaml and compose.db.yaml, got: %v", files)
	}
	if foundCache {
		t.Errorf("compose.cache.yaml should NOT be included when filtering /app,/db, got: %v", files)
	}
}
