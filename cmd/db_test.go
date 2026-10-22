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

func TestWriteEnvFile_Redis(t *testing.T) {
	workDir := t.TempDir()

	if err := writeEnvFile(workDir, "redis", "redis", "myapp", "myapp", "redispass123"); err != nil {
		t.Fatalf("writeEnvFile failed: %v", err)
	}

	expectedPath := filepath.Join(workDir, "config", "env", "redis.myapp.env")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("expected env file at %s, got error: %v", expectedPath, err)
	}

	content := string(data)
	if !strings.Contains(content, "DB_ENGINE=redis") {
		t.Errorf("expected DB_ENGINE=redis, got: %s", content)
	}
	if !strings.Contains(content, "DB_PORT=6379") {
		t.Errorf("expected DB_PORT=6379, got: %s", content)
	}
	if !strings.Contains(content, "DB_URL=redis://myapp:redispass123@redis:6379/0") {
		t.Errorf("expected DB_URL for redis, got: %s", content)
	}
	if !strings.Contains(content, "REDIS_URL=redis://myapp:redispass123@redis:6379/0") {
		t.Errorf("expected REDIS_URL for redis, got: %s", content)
	}

	// Test password rotation for redis
	if err := writePasswdEnvFile(workDir, "redis", "redis", "myapp", "newpass789"); err != nil {
		t.Fatalf("writePasswdEnvFile failed: %v", err)
	}

	dataAfter, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed reading updated env file: %v", err)
	}
	contentAfter := string(dataAfter)
	if !strings.Contains(contentAfter, "DB_PASSWORD=newpass789") {
		t.Errorf("expected rotated password, got: %s", contentAfter)
	}
	if !strings.Contains(contentAfter, "REDIS_URL=redis://myapp:newpass789@redis:6379/0") {
		t.Errorf("expected rotated REDIS_URL, got: %s", contentAfter)
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

