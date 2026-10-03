package backup_test

import (
	"os"
	"path/filepath"
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
