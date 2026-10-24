package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteConfigManagement(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test SaveConfig
	cfg := &Config{
		Host:    "prod.example.com",
		Path:    "~/oopsbox",
		Zone:    "asia-southeast1-b",
		Project: "my-gcp-project",
		Tunnel: TunnelManifest{
			Services: []string{"mysql", "redis"},
		},
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
	if loaded.Host != "prod.example.com" || loaded.Path != "~/oopsbox" || loaded.Zone != "asia-southeast1-b" || loaded.Project != "my-gcp-project" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
	if len(loaded.Tunnel.Services) != 2 || loaded.Tunnel.Services[0] != "mysql" || loaded.Tunnel.Services[1] != "redis" {
		t.Fatalf("unexpected tunnel services: %v", loaded.Tunnel.Services)
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

	// 5. Test InitCapsuleBlueprints generates manifests, .env, and config/
	created, err := InitCapsuleBlueprints(tempDir, "newbox")
	if err != nil {
		t.Fatalf("InitCapsuleBlueprints failed: %v", err)
	}
	if len(created) == 0 {
		t.Errorf("expected created items, got 0")
	}

	capsuleDir := filepath.Join(tempDir, "@newbox")
	for _, expectedFile := range []string{"compose.yaml", "oops.yaml", ".oopsallow", ".env"} {
		if _, err := os.Stat(filepath.Join(capsuleDir, expectedFile)); err != nil {
			t.Errorf("expected %s to be created in capsule: %v", expectedFile, err)
		}
	}

	// Verify Remote Identity Invariant: tld == newbox and network.subnet default
	oopsYamlContent, err := os.ReadFile(filepath.Join(capsuleDir, "oops.yaml"))
	if err != nil {
		t.Fatalf("failed reading capsule oops.yaml: %v", err)
	}
	if !strings.Contains(string(oopsYamlContent), "tld: newbox") {
		t.Errorf("expected capsule oops.yaml to have 'tld: newbox', got:\n%s", string(oopsYamlContent))
	}
	if !strings.Contains(string(oopsYamlContent), "subnet: 10.240.0.0/16") {
		t.Errorf("expected capsule oops.yaml to have 'subnet: 10.240.0.0/16', got:\n%s", string(oopsYamlContent))
	}

	for _, expectedDir := range []string{"config", "data", "backups"} {
		if fi, err := os.Stat(filepath.Join(capsuleDir, expectedDir)); err != nil || !fi.IsDir() {
			t.Errorf("expected %s directory in capsule, got: %v", expectedDir, err)
		}
	}

	// 6. Test SaveConfig with empty tunnel writes commented template
	if err := SaveConfig(tempDir, "empty-tunnel", &Config{Host: "host.local", Path: "~/oopsbox"}); err != nil {
		t.Fatalf("SaveConfig empty-tunnel failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(tempDir, "@empty-tunnel", "remote.yaml"))
	if err != nil {
		t.Fatalf("failed reading remote.yaml: %v", err)
	}
	if !strings.Contains(string(content), "# tunnel:") || !strings.Contains(string(content), "#   services:") {
		t.Errorf("expected commented tunnel template in remote.yaml, got:\n%s", string(content))
	}
}
