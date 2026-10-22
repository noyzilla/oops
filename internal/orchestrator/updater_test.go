package orchestrator_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/noyzilla/oops/internal/orchestrator"
)

func TestParseDelay(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"0s", 0, false},
		{"5", 5 * time.Second, false},
		{"10s", 10 * time.Second, false},
		{"1m", 1 * time.Minute, false},
		{"2m30s", 150 * time.Second, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := orchestrator.ParseDelay(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("ParseDelay(%q) error = %v, expected error = %v", tt.input, err, tt.hasError)
		}
		if got != tt.expected {
			t.Errorf("ParseDelay(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseDurationWithDefault(t *testing.T) {
	tests := []struct {
		input       string
		defaultDur  time.Duration
		expected    time.Duration
	}{
		{"", 30 * time.Second, 30 * time.Second},
		{"45", 30 * time.Second, 45 * time.Second},
		{"45s", 30 * time.Second, 45 * time.Second},
		{"1m", 30 * time.Second, 1 * time.Minute},
		{"10m", 30 * time.Second, 10 * time.Minute},
		{"invalid", 10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		got := orchestrator.ParseDurationWithDefault(tt.input, tt.defaultDur)
		if got != tt.expected {
			t.Errorf("ParseDurationWithDefault(%q, %v) = %v, expected %v", tt.input, tt.defaultDur, got, tt.expected)
		}
	}
}

func TestFindEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"
	_ = os.WriteFile(envPath, []byte("OOPS_SECRET=test-secret"), 0644)

	// Compose file in sub-directory services/edge/compose.yaml
	servicesEdgeDir := tmpDir + "/services/edge"
	_ = os.MkdirAll(servicesEdgeDir, 0755)
	composeFile := servicesEdgeDir + "/compose.yaml"
	_ = os.WriteFile(composeFile, []byte("services: {}"), 0644)

	found := orchestrator.FindEnvFile(composeFile)
	if found != envPath {
		t.Errorf("expected FindEnvFile to find %q, got %q", envPath, found)
	}
}

func TestComposeCommandEnv(t *testing.T) {
	env := orchestrator.ComposeCommandEnv()
	foundOrphans := false
	foundUID := false
	foundGID := false
	for _, entry := range env {
		if entry == "COMPOSE_IGNORE_ORPHANS=True" {
			foundOrphans = true
		}
		if strings.HasPrefix(entry, "UID=") {
			foundUID = true
		}
		if strings.HasPrefix(entry, "GID=") {
			foundGID = true
		}
	}
	if !foundOrphans {
		t.Fatalf("expected COMPOSE_IGNORE_ORPHANS=True in ComposeCommandEnv(), got %v", env)
	}
	if !foundUID {
		t.Fatalf("expected UID=... in ComposeCommandEnv(), got %v", env)
	}
	if !foundGID {
		t.Fatalf("expected GID=... in ComposeCommandEnv(), got %v", env)
	}
}

func TestSyncUserUIDGIDToEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// 1. Initial creation when .env does not exist
	if err := orchestrator.SyncUserUIDGIDToEnvFile(envPath); err != nil {
		t.Fatalf("failed syncing to non-existent .env: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	wantUID := fmt.Sprintf("UID=%d", os.Getuid())
	wantGID := fmt.Sprintf("GID=%d", os.Getgid())
	if !strings.Contains(string(content), wantUID) || !strings.Contains(string(content), wantGID) {
		t.Errorf("expected .env to contain %q and %q, got:\n%s", wantUID, wantGID, string(content))
	}

	// 2. Overwrite existing outdated UID/GID from another environment (e.g. from macOS 501/20)
	dummyEnv := "FOO=bar\nUID=9999\nBAZ=qux\nGID=8888\n"
	if err := os.WriteFile(envPath, []byte(dummyEnv), 0644); err != nil {
		t.Fatal(err)
	}

	if err := orchestrator.SyncUserUIDGIDToEnvFile(envPath); err != nil {
		t.Fatalf("failed updating existing .env: %v", err)
	}

	updated, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedStr := string(updated)

	if !strings.Contains(updatedStr, "FOO=bar") || !strings.Contains(updatedStr, "BAZ=qux") {
		t.Errorf("expected preserved FOO and BAZ variables, got:\n%s", updatedStr)
	}
	if strings.Contains(updatedStr, "UID=9999") || strings.Contains(updatedStr, "GID=8888") {
		t.Errorf("expected outdated UID=9999 and GID=8888 to be overwritten, got:\n%s", updatedStr)
	}
	if !strings.Contains(updatedStr, wantUID) || !strings.Contains(updatedStr, wantGID) {
		t.Errorf("expected new %q and %q, got:\n%s", wantUID, wantGID, updatedStr)
	}
}

