package box

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRandomSecret(t *testing.T) {
	s1, err := GenerateRandomSecret(48)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s1) != 48 {
		t.Errorf("expected secret length 48, got %d", len(s1))
	}

	s2, err := GenerateRandomSecret(48)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1 == s2 {
		t.Errorf("expected generated secrets to be unique, got duplicate")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("skipping test: no user home directory")
	}

	path := "~/oopsbox"
	expanded := ExpandHome(path)
	expected := filepath.Join(home, "oopsbox")
	if expanded != expected {
		t.Errorf("expected %s, got %s", expected, expanded)
	}

	raw := "/tmp/test"
	if ExpandHome(raw) != raw {
		t.Errorf("expected unchanged path for /tmp/test, got %s", ExpandHome(raw))
	}
}

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	files := []struct {
		Name string
		Body string
	}{
		{"oops-main/oopsbox/stacks/edge/compose.yml", "services:\n  caddy-proxy:\n    image: caddy"},
		{"oops-main/oopsbox/oopsbox.yml.example", "engine:\n  type: auto"},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.Name,
			Mode: 0644,
			Size: int64(len(f.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(f.Body)); err != nil {
			t.Fatalf("failed to write tar body: %v", err)
		}
	}

	tw.Close()
	gzw.Close()

	tmpDir, err := os.MkdirTemp("", "extract_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := ExtractTarGz(bytes.NewReader(buf.Bytes()), tmpDir, "oopsbox"); err != nil {
		t.Fatalf("ExtractTarGz failed: %v", err)
	}

	composeFile := filepath.Join(tmpDir, "stacks", "edge", "compose.yml")
	if _, err := os.Stat(composeFile); err != nil {
		t.Errorf("expected extracted file at %s, got error: %v", composeFile, err)
	}

	cfgFile := filepath.Join(tmpDir, "oopsbox.yml.example")
	if _, err := os.Stat(cfgFile); err != nil {
		t.Errorf("expected extracted file at %s, got error: %v", cfgFile, err)
	}
}

func TestInitWorkspaceEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write .env.example
	exampleContent := `OOPS_SECRET=
MYSQL_ROOT_PASSWORD=secret
POSTGRES_PASSWORD=secret
REDIS_PASSWORD=secret
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.example"), []byte(exampleContent), 0644); err != nil {
		t.Fatalf("failed to write .env.example: %v", err)
	}

	if err := InitWorkspaceEnv(tmpDir); err != nil {
		t.Fatalf("InitWorkspaceEnv failed: %v", err)
	}

	envBytes, err := os.ReadFile(filepath.Join(tmpDir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	envStr := string(envBytes)

	if strings.Contains(envStr, "OOPS_SECRET=\n") || strings.Contains(envStr, "OOPS_SECRET=\"\"") {
		t.Errorf(".env has empty OOPS_SECRET: %s", envStr)
	}
	if strings.Contains(envStr, "MYSQL_ROOT_PASSWORD=secret") {
		t.Errorf(".env has placeholder db password: %s", envStr)
	}
}

func TestLoadOopsboxConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cfg_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Default config when missing
	cfg, err := LoadOopsboxConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Engine.Type != "auto" || cfg.DNS.TLD != "oops" {
		t.Errorf("expected default auto/oops, got %s/%s", cfg.Engine.Type, cfg.DNS.TLD)
	}

	// Custom config
	yamlContent := `engine:
  type: orbstack
dns:
  tld: dev
`
	if err := os.WriteFile(filepath.Join(tmpDir, "oopsbox.yml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write oopsbox.yml: %v", err)
	}

	cfg, err = LoadOopsboxConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Engine.Type != "orbstack" || cfg.DNS.TLD != "dev" {
		t.Errorf("expected orbstack/dev, got %s/%s", cfg.Engine.Type, cfg.DNS.TLD)
	}
}
