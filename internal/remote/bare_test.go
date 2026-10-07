package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePostReceiveHook(t *testing.T) {
	boxPath := "/var/oopsbox"
	hook := GeneratePostReceiveHook(boxPath)

	if !strings.Contains(hook, "OOPSBOX_DIR=\"/var/oopsbox\"") {
		t.Errorf("expected OOPSBOX_DIR to be set to /var/oopsbox, got:\n%s", hook)
	}
	if !strings.Contains(hook, "DO_DEPLOY=1") {
		t.Errorf("expected deploy option check in hook, got:\n%s", hook)
	}
	if !strings.Contains(hook, "oops up -C \"$OOPSBOX_DIR\"") {
		t.Errorf("expected oops up execution in hook, got:\n%s", hook)
	}
}

func TestInitBareRepo(t *testing.T) {
	tempDir := t.TempDir()
	barePath := filepath.Join(tempDir, "testbox.git")
	boxPath := filepath.Join(tempDir, "oopsbox")

	if err := os.MkdirAll(boxPath, 0755); err != nil {
		t.Fatalf("failed creating temp box path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boxPath, "oopsbox.yml"), []byte("engine: docker"), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	if err := InitBareRepo(barePath, boxPath); err != nil {
		t.Fatalf("InitBareRepo failed: %v", err)
	}

	hookFile := filepath.Join(barePath, "hooks", "post-receive")
	info, err := os.Stat(hookFile)
	if err != nil {
		t.Fatalf("post-receive hook missing: %v", err)
	}

	// Perm check (must be executable)
	if info.Mode()&0111 == 0 {
		t.Errorf("post-receive hook is not executable, mode: %v", info.Mode())
	}

	content, _ := os.ReadFile(hookFile)
	if !strings.Contains(string(content), "GIT_WORK_TREE") {
		t.Errorf("post-receive hook content invalid:\n%s", string(content))
	}
}
