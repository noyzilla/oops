package key

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureKeyPairAndGenerate(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")

	pubStr, gen, err := EnsureKeyPair(keyPath)
	if err != nil {
		t.Fatalf("EnsureKeyPair failed: %v", err)
	}
	if !gen {
		t.Errorf("expected generated=true for new key")
	}
	if !strings.HasPrefix(pubStr, "ssh-ed25519 ") {
		t.Errorf("unexpected public key format: %s", pubStr)
	}

	pubStr2, gen2, err2 := EnsureKeyPair(keyPath)
	if err2 != nil {
		t.Fatalf("EnsureKeyPair (second call) failed: %v", err2)
	}
	if gen2 {
		t.Errorf("expected generated=false for existing key")
	}
	if pubStr != pubStr2 {
		t.Errorf("public keys mismatch: %s vs %s", pubStr, pubStr2)
	}
}

func TestGenerateKeyPairReset(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")

	pub1, err := GenerateKeyPair(keyPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair 1 failed: %v", err)
	}

	pub2, err := GenerateKeyPair(keyPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair 2 failed: %v", err)
	}

	if pub1 == pub2 {
		t.Errorf("expected new key pair after reset, got identical public key")
	}
}

func TestSetKeyPair(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")

	pubGen, err := GenerateKeyPair(keyPath)
	if err != nil {
		t.Fatalf("failed generating reference key: %v", err)
	}

	privBytes, _ := os.ReadFile(keyPath)

	dir2 := t.TempDir()
	keyPath2 := filepath.Join(dir2, "id_ed25519")

	pubSet, err := SetKeyPair(keyPath2, string(privBytes))
	if err != nil {
		t.Fatalf("SetKeyPair failed: %v", err)
	}
	if pubGen != pubSet {
		t.Errorf("SetKeyPair public key mismatch: got %s want %s", pubSet, pubGen)
	}
}

func TestGetAddDeployKeyURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:noyzilla/oopsbox.git", "https://github.com/noyzilla/oopsbox/settings/keys/new"},
		{"https://github.com/noyzilla/oopsbox.git", "https://github.com/noyzilla/oopsbox/settings/keys/new"},
		{"git@github.com:owner/subrepo", "https://github.com/owner/subrepo/settings/keys/new"},
		{"git@gitlab.com:group/project.git", "https://gitlab.com/group/project/-/settings/repository"},
		{"https://customgit.com/group/project.git", ""},
	}

	for _, tt := range tests {
		got := GetAddDeployKeyURL(tt.url)
		if got != tt.want {
			t.Errorf("GetAddDeployKeyURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
