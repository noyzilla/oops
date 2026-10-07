package cmd

import (
	"os"
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
