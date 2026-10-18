package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/internal/docker"
)

func TestWriteEnvFileUsesWorkDir(t *testing.T) {
	workDir := t.TempDir()
	otherDir := t.TempDir()

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWD)

	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}

	if err := writeEnvFile(workDir, "mysql", "mysql", "appdb", "appuser", "secret123"); err != nil {
		t.Fatalf("writeEnvFile failed: %v", err)
	}

	expectedPath := filepath.Join(workDir, "config", "env", "mysql.appuser.env")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected env file at %s, got error: %v", expectedPath, err)
	}
	if !strings.Contains(string(data), "DB_PASSWORD=secret123") {
		t.Errorf("unexpected env content: %s", string(data))
	}

	wrongPath := filepath.Join(otherDir, "config", "env", "mysql.appuser.env")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("should not have created env file in caller's current directory %s", wrongPath)
	}

	// Test password rotation updates the file in workDir
	if err := writePasswdEnvFile(workDir, "mysql", "mysql", "appuser", "rotated456"); err != nil {
		t.Fatalf("writePasswdEnvFile failed: %v", err)
	}

	dataAfter, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed reading updated env file: %v", err)
	}
	if !strings.Contains(string(dataAfter), "DB_PASSWORD=rotated456") {
		t.Errorf("expected rotated password in %s, got:\n%s", expectedPath, string(dataAfter))
	}
}

func TestResolveBackupDirForWorkDir(t *testing.T) {
	t.Setenv("OOPS_BACKUP_DIR", "")
	cfg := &docker.OopsConfig{}
	cfg.Backup.Dir = "./backups"

	got := resolveBackupDirForWorkDir("/opt/oopsbox", cfg)
	want := filepath.Join("/opt/oopsbox", "backups")
	if got != want {
		t.Errorf("resolveBackupDirForWorkDir() = %q, want %q", got, want)
	}

	// Absolute path should not be prefixed with workDir
	cfg.Backup.Dir = "/mnt/backups"
	gotAbs := resolveBackupDirForWorkDir("/opt/oopsbox", cfg)
	if gotAbs != "/mnt/backups" {
		t.Errorf("resolveBackupDirForWorkDir() = %q, want /mnt/backups", gotAbs)
	}
}

