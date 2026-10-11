package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestToGitRemoteNameAndLogical(t *testing.T) {
	if got := toGitRemoteName("prod"); got != "oops-prod" {
		t.Errorf("toGitRemoteName('prod') = %q, want 'oops-prod'", got)
	}
	if got := toGitRemoteName("oops-prod"); got != "oops-prod" {
		t.Errorf("toGitRemoteName('oops-prod') = %q, want 'oops-prod'", got)
	}
	if got := toLogicalRemoteName("oops-prod"); got != "prod" {
		t.Errorf("toLogicalRemoteName('oops-prod') = %q, want 'prod'", got)
	}
	if got := toLogicalRemoteName("prod"); got != "prod" {
		t.Errorf("toLogicalRemoteName('prod') = %q, want 'prod'", got)
	}
}

func TestDeriveRemoteName(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. First remote defaults to prod
	name, err := deriveRemoteName(tmpDir, "user@1.2.3.4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "prod" {
		t.Errorf("deriveRemoteName first time = %q, want 'prod'", name)
	}

	// 2. Explicit flag -r staging
	name, err = deriveRemoteName(tmpDir, "user@1.2.3.4", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "staging" {
		t.Errorf("deriveRemoteName with explicit flag = %q, want 'staging'", name)
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
	if err := os.MkdirAll(filepath.Join(tmpDir, "stacks"), 0755); err != nil {
		t.Fatalf("failed creating stacks dir: %v", err)
	}

	got, err := RequireOopsboxWorkspace(tmpDir)
	if err != nil {
		t.Errorf("RequireOopsboxWorkspace(%q) unexpected error: %v", tmpDir, err)
	}
	if got == "" {
		t.Errorf("RequireOopsboxWorkspace(%q) returned empty string", tmpDir)
	}
}

func TestRequireGitOopsboxWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "stacks"), 0755); err != nil {
		t.Fatalf("failed creating stacks dir: %v", err)
	}

	// 1. Valid compose stacks BUT no .git -> Expect error
	_, err := RequireGitOopsboxWorkspace(tmpDir)
	if err == nil {
		t.Errorf("RequireGitOopsboxWorkspace(%q) expected error when missing .git, got nil", tmpDir)
	}

	// 2. Add valid git repo via git init -> Expect success
	if out, err := exec.Command("git", "-C", tmpDir, "init").CombinedOutput(); err != nil {
		t.Fatalf("failed git init: %s (%v)", string(out), err)
	}
	got, err := RequireGitOopsboxWorkspace(tmpDir)
	if err != nil {
		t.Errorf("RequireGitOopsboxWorkspace(%q) unexpected error with .git: %v", tmpDir, err)
	}
	if got == "" {
		t.Errorf("RequireGitOopsboxWorkspace(%q) returned empty string", tmpDir)
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

