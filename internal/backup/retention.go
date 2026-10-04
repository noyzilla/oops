package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ParseRetention parses retention strings like "7d", "14d", "30", "24h"
func ParseRetention(retentionStr string) (time.Duration, error) {
	trimmed := strings.TrimSpace(retentionStr)
	if trimmed == "" {
		return 7 * 24 * time.Hour, nil
	}

	if strings.HasSuffix(trimmed, "d") {
		dayStr := strings.TrimSuffix(trimmed, "d")
		days, err := strconv.Atoi(dayStr)
		if err != nil {
			return 0, fmt.Errorf("invalid retention days %q: %w", retentionStr, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	if days, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(days) * 24 * time.Hour, nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid retention duration %q: %w", retentionStr, err)
	}
	return d, nil
}

// PruneOldBackups deletes all backup files in backupDir older than retention threshold
func PruneOldBackups(backupDir string, retention time.Duration) ([]string, error) {
	return pruneMatchingBackups(backupDir, retention, func(name string) bool {
		return strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".rdb") || strings.HasSuffix(name, ".tar.gz")
	})
}

// PruneDBBackups deletes database backup files in backupDir older than retention threshold
func PruneDBBackups(backupDir string, retention time.Duration) ([]string, error) {
	return pruneMatchingBackups(backupDir, retention, func(name string) bool {
		return strings.HasSuffix(name, ".sql.gz") || strings.HasSuffix(name, ".rdb.gz") || strings.HasSuffix(name, ".rdb")
	})
}

// PruneDataBackups deletes data volume/filesystem archives in backupDir older than retention threshold
func PruneDataBackups(backupDir string, retention time.Duration) ([]string, error) {
	return pruneMatchingBackups(backupDir, retention, func(name string) bool {
		return strings.HasPrefix(name, "data_") && strings.HasSuffix(name, ".tar.gz")
	})
}

func pruneMatchingBackups(backupDir string, retention time.Duration, filter func(string) bool) ([]string, error) {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup dir %s: %w", backupDir, err)
	}

	threshold := time.Now().Add(-retention)
	var deletedFiles []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !filter(name) {
			continue
		}

		filePath := filepath.Join(backupDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(threshold) {
			if err := os.Remove(filePath); err == nil {
				deletedFiles = append(deletedFiles, name)
			}
		}
	}

	return deletedFiles, nil
}
