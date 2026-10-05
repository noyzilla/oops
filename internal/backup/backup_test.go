package backup_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/noyzilla/oops/internal/backup"
)

func TestParseRetention(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"", 7 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"14d", 14 * 24 * time.Hour, false},
		{"30", 30 * 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := backup.ParseRetention(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("ParseRetention(%q) error = %v, expected error = %v", tt.input, err, tt.hasError)
		}
		if got != tt.expected {
			t.Errorf("ParseRetention(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestPruneOldBackups(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-backup-prune-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create old file
	oldFile := filepath.Join(tmpDir, "mysql_backup_20200101_000000.sql.gz")
	os.WriteFile(oldFile, []byte("fake dump"), 0644)
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create recent file
	recentFile := filepath.Join(tmpDir, "mysql_backup_20261001_000000.sql.gz")
	os.WriteFile(recentFile, []byte("fake dump"), 0644)

	deleted, err := backup.PruneOldBackups(tmpDir, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error pruning backups: %v", err)
	}

	if len(deleted) != 1 || deleted[0] != "mysql_backup_20200101_000000.sql.gz" {
		t.Errorf("unexpected deleted files: %v", deleted)
	}

	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Errorf("recent file was incorrectly pruned")
	}
}

func TestExecuteDataBackup(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "workspace")
	backupDir := filepath.Join(tmpDir, "backups")

	_ = os.MkdirAll(filepath.Join(workDir, "stacks"), 0755)
	_ = os.MkdirAll(filepath.Join(workDir, "data", "uploads"), 0755)
	_ = os.MkdirAll(filepath.Join(workDir, "data", "caddy"), 0755)

	_ = os.WriteFile(filepath.Join(workDir, "data", "uploads", "sample.png"), []byte("png image data"), 0644)
	_ = os.WriteFile(filepath.Join(workDir, "data", "caddy", "root.crt"), []byte("cert data"), 0644)

	// Write oops.yml config
	oopsYaml := `
backups:
  retention: 7d
  data:
    - name: uploads
      paths:
        - data/uploads
    - name: certs
      path: data/caddy
`
	_ = os.WriteFile(filepath.Join(workDir, "oops.yml"), []byte(oopsYaml), 0644)

	// Execute data backup
	err := backup.ExecuteDataBackup(t.Context(), workDir, backupDir, nil, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("ExecuteDataBackup failed: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup dir: %v", err)
	}

	foundUploads := false
	foundCerts := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "data_uploads_") && strings.HasSuffix(e.Name(), ".tar.gz") {
			foundUploads = true
		}
		if strings.HasPrefix(e.Name(), "data_certs_") && strings.HasSuffix(e.Name(), ".tar.gz") {
			foundCerts = true
		}
	}

	if !foundUploads {
		t.Errorf("expected data_uploads archive in backup dir, found entries: %v", entries)
	}
	if !foundCerts {
		t.Errorf("expected data_certs archive in backup dir, found entries: %v", entries)
	}

	// Read and verify embedded manifest in data_uploads
	var uploadArchivePath string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "data_uploads_") {
			uploadArchivePath = filepath.Join(backupDir, e.Name())
			break
		}
	}
	manifest, err := backup.ReadDataManifestFromArchive(uploadArchivePath)
	if err != nil {
		t.Fatalf("failed to read manifest from data archive: %v", err)
	}
	if manifest == nil || manifest.Name != "uploads" {
		t.Errorf("expected manifest with name 'uploads', got %+v", manifest)
	}

	// Test ExecuteRestoreData (dry-run)
	err = backup.ExecuteRestoreData(t.Context(), workDir, backupDir, "uploads", true, true)
	if err != nil {
		t.Fatalf("ExecuteRestoreData dry-run failed: %v", err)
	}

	// Remove file to test actual restore
	sampleFile := filepath.Join(workDir, "data", "uploads", "sample.png")
	_ = os.Remove(sampleFile)

	err = backup.ExecuteRestoreData(t.Context(), workDir, backupDir, "uploads", false, true)
	if err != nil {
		t.Fatalf("ExecuteRestoreData actual restore failed: %v", err)
	}

	if _, err := os.Stat(sampleFile); os.IsNotExist(err) {
		t.Errorf("expected sample.png to be restored, but not found")
	}
}

func TestSQLManifestReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	sqlFile := filepath.Join(tmpDir, "test.sql")

	f, err := os.Create(sqlFile)
	if err != nil {
		t.Fatalf("failed to create sql file: %v", err)
	}

	backup.WriteSQLHeader(f, backup.DBManifest{
		Engine:    "mysql",
		Container: "mysql-db",
		Database:  "app_db",
	})
	f.WriteString("CREATE TABLE users (id INT);\n")
	f.Close()

	manifest, err := backup.ReadDBManifestFromFile(sqlFile)
	if err != nil {
		t.Fatalf("failed to read db manifest: %v", err)
	}
	if manifest == nil {
		t.Fatalf("expected manifest, got nil")
	}
	if manifest.Engine != "mysql" || manifest.Container != "mysql-db" || manifest.Database != "app_db" {
		t.Errorf("manifest mismatch: %+v", manifest)
	}
}

func TestFindLatestDBBackup_Redis(t *testing.T) {
	tmpDir := t.TempDir()
	redisFile := filepath.Join(tmpDir, "redis_backup_20261004_120000.rdb.gz")
	_ = os.WriteFile(redisFile, []byte("fake rdb gz"), 0644)

	latest, err := backup.FindLatestDBBackup(tmpDir, "redis", "")
	if err != nil {
		t.Fatalf("FindLatestDBBackup failed for redis: %v", err)
	}
	if latest != redisFile {
		t.Errorf("expected %s, got %s", redisFile, latest)
	}
}

type errReader struct{}

func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read failure")
}

func TestReadSQLHeader_ScannerError(t *testing.T) {
	manifest, err := backup.ReadSQLHeader(&errReader{})
	if err == nil {
		t.Fatalf("expected error from ReadSQLHeader on bad reader, got nil error (manifest: %+v)", manifest)
	}
	if !strings.Contains(err.Error(), "simulated read failure") {
		t.Errorf("unexpected error message: %v", err)
	}
}
