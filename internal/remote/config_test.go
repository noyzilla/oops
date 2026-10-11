package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteConfigManagement(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test SaveConfig
	cfg := &Config{
		Host: "prod.example.com",
		Path: "~/oopsbox",
	}
	if err := SaveConfig(tempDir, "prod", cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify capsule directory structure
	capsulePath := filepath.Join(tempDir, "@prod")
	if fi, err := os.Stat(capsulePath); err != nil || !fi.IsDir() {
		t.Fatalf("expected @prod directory to exist")
	}

	cfgFile := filepath.Join(capsulePath, "remote.yaml")
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected remote.yaml to exist in @prod")
	}

	// 2. Test LoadConfig
	loaded, err := LoadConfig(tempDir, "prod")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Host != "prod.example.com" || loaded.Path != "~/oopsbox" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}

	// Also test LoadConfig with @ prefix
	loadedWithAt, err := LoadConfig(tempDir, "@prod")
	if err != nil {
		t.Fatalf("LoadConfig with @ failed: %v", err)
	}
	if loadedWithAt.Host != "prod.example.com" {
		t.Fatalf("unexpected loaded host: %s", loadedWithAt.Host)
	}

	// 3. Test ListRemotes
	_ = SaveConfig(tempDir, "staging", &Config{Host: "staging.example.com", Path: "~/staging"})
	remotes, err := ListRemotes(tempDir)
	if err != nil {
		t.Fatalf("ListRemotes failed: %v", err)
	}
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %v", remotes)
	}

	// 4. Test RemoveRemote
	if err := RemoveRemote(tempDir, "staging"); err != nil {
		t.Fatalf("RemoveRemote failed: %v", err)
	}
	remotesAfter, _ := ListRemotes(tempDir)
	if len(remotesAfter) != 1 || remotesAfter[0] != "prod" {
		t.Fatalf("expected only prod remaining, got %v", remotesAfter)
	}
}
