package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DumpMySQL executes mysqldump inside container and writes to gzip writer
func DumpMySQL(ctx context.Context, containerTarget string, w io.Writer) error {
	rootPass := os.Getenv("MYSQL_ROOT_PASSWORD")
	var cmdArgs []string
	if rootPass != "" {
		cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "-p" + rootPass, "--all-databases"}
	} else {
		cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "--all-databases"}
	}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DumpPostgres executes pg_dumpall inside container and writes to gzip writer
func DumpPostgres(ctx context.Context, containerTarget string, w io.Writer) error {
	pgUser := os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		pgUser = "postgres"
	}

	cmdArgs := []string{"exec", "-i", containerTarget, "pg_dumpall", "-U", pgUser}
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecuteBackup runs dump for matching database containers and prunes old archives
func ExecuteBackup(ctx context.Context, backupDir string, targets []string, retention time.Duration) error {
	if backupDir == "" {
		backupDir = "./backups"
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	if len(targets) == 0 {
		targets = []string{"mysql", "postgres"}
	}

	for _, t := range targets {
		var dumpErr error
		fileName := fmt.Sprintf("%s_backup_%s.sql.gz", t, timestamp)
		filePath := filepath.Join(backupDir, fileName)

		f, err := os.Create(filePath)
		if err != nil {
			log.Printf("Failed to create file %s: %v", filePath, err)
			continue
		}

		gzWriter := gzip.NewWriter(f)

		if strings.Contains(t, "mysql") {
			log.Printf("==> Creating MySQL backup: %s...", fileName)
			dumpErr = DumpMySQL(ctx, t, gzWriter)
		} else if strings.Contains(t, "postgres") || strings.Contains(t, "pg") {
			log.Printf("==> Creating Postgres backup: %s...", fileName)
			dumpErr = DumpPostgres(ctx, t, gzWriter)
		} else {
			log.Printf("Skipping unrecognized engine target: %s", t)
			gzWriter.Close()
			f.Close()
			os.Remove(filePath)
			continue
		}

		gzWriter.Close()
		f.Close()

		if dumpErr != nil {
			log.Printf("Error dumping %s: %v", t, dumpErr)
			os.Remove(filePath)
		} else {
			log.Printf("Successfully created backup: %s", filePath)
		}
	}

	// Prune old backups
	deleted, err := PruneOldBackups(backupDir, retention)
	if err != nil {
		log.Printf("Warning during backup pruning: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("Pruned %d expired backup archives: %v", len(deleted), deleted)
	}

	return nil
}
