package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsKnownRemoteOrHost(t *testing.T) {
	tmpDir := t.TempDir()
	prodCapsule := filepath.Join(tmpDir, "@prod")
	if err := os.MkdirAll(prodCapsule, 0755); err != nil {
		t.Fatalf("failed creating @prod dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(prodCapsule, "remote.yaml"), []byte("host: user@1.2.3.4\npath: ~/box\n"), 0644)

	if !isKnownRemoteOrHost(tmpDir, "@prod") {
		t.Errorf("expected @prod to be known remote")
	}
	if !isKnownRemoteOrHost(tmpDir, "prod") {
		t.Errorf("expected prod to be known remote")
	}
	if !isKnownRemoteOrHost(tmpDir, "user@server.com") {
		t.Errorf("expected user@server.com to be recognized as SSH target")
	}
	if isKnownRemoteOrHost(tmpDir, "unknown") {
		t.Errorf("expected unknown to not be known remote")
	}
}

func TestParseRemoteConnectTarget(t *testing.T) {
	tests := []struct {
		input       string
		wantTarget  string
		wantCapsule string
	}{
		{"@prod", "prod", "prod"},
		{"prod", "prod", "prod"},
		{"user@1.2.3.4", "user@1.2.3.4", "1.2.3.4"},
		{"@user@1.2.3.4", "user@1.2.3.4", "1.2.3.4"},
		{"root@vps.example.com:22", "root@vps.example.com:22", "vps.example.com"},
		{"@staging-box", "staging-box", "staging-box"},
	}

	for _, tt := range tests {
		gotTarget, gotCapsule := parseRemoteConnectTarget(tt.input)
		if gotTarget != tt.wantTarget || gotCapsule != tt.wantCapsule {
			t.Errorf("parseRemoteConnectTarget(%q) = (%q, %q); want (%q, %q)",
				tt.input, gotTarget, gotCapsule, tt.wantTarget, tt.wantCapsule)
		}
	}
}

func TestCapsuleHasContent(t *testing.T) {
	tmpDir := t.TempDir()
	capsuleDir := filepath.Join(tmpDir, "@prod")
	_ = os.MkdirAll(capsuleDir, 0755)

	// Initially empty (or only remote.yaml)
	if capsuleHasContent(capsuleDir) {
		t.Errorf("expected empty capsule to return false")
	}

	_ = os.WriteFile(filepath.Join(capsuleDir, "remote.yaml"), []byte("host: x\n"), 0644)
	if capsuleHasContent(capsuleDir) {
		t.Errorf("expected capsule with only remote.yaml to return false")
	}

	// Add .env
	_ = os.WriteFile(filepath.Join(capsuleDir, ".env"), []byte("FOO=BAR\n"), 0644)
	if !capsuleHasContent(capsuleDir) {
		t.Errorf("expected capsule with .env to return true")
	}
}

func TestRequireOopsboxWorkspace_OutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := RequireOopsboxWorkspace(tmpDir)
	if err == nil {
		t.Errorf("RequireOopsboxWorkspace(%q) expected error when outside workspace, got nil", tmpDir)
	}
}

func TestRequireOopsboxWorkspace_ValidWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "services"), 0755); err != nil {
		t.Fatalf("failed creating services dir: %v", err)
	}

	got, err := RequireOopsboxWorkspace(tmpDir)
	if err != nil {
		t.Errorf("RequireOopsboxWorkspace(%q) unexpected error: %v", tmpDir, err)
	}
	if got == "" {
		t.Errorf("RequireOopsboxWorkspace(%q) returned empty string", tmpDir)
	}
}

func TestRemoteCapsuleAddAndList(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte("services:\n  app:\n    image: test\n"), 0644)

	// Add remote capsule @prod
	prodCapsule := filepath.Join(tmpDir, "@prod")
	if err := os.MkdirAll(prodCapsule, 0755); err != nil {
		t.Fatalf("failed creating @prod dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(prodCapsule, "remote.yaml"), []byte("host: user@1.2.3.4\npath: ~/box\n"), 0644)

	// Test resolveDefaultServer finds prod
	defServer := resolveDefaultServer(tmpDir)
	if defServer != "prod" {
		t.Errorf("expected default server 'prod', got %s", defServer)
	}

	// Test resolveSSHTarget resolves host from remote.yaml
	target := resolveSSHTarget(tmpDir, "prod")
	if target != "user@1.2.3.4" {
		t.Errorf("expected ssh target 'user@1.2.3.4', got %s", target)
	}
}

