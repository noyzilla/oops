package backup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/docker"
)

// ExecuteRestoreDB restores a .sql or .sql.gz database dump into a running database container
func ExecuteRestoreDB(ctx context.Context, backupDir string, engineSpec string, dbName string, backupFilePath string, force bool) error {
	parts := strings.SplitN(engineSpec, ":", 2)
	engine := strings.ToLower(parts[0])
	containerTarget := engine
	if len(parts) == 2 {
		containerTarget = parts[1]
	}

	if containerTarget == "pg" || containerTarget == "postgres" {
		containerTarget = "postgres"
	} else if containerTarget == "mysql" {
		containerTarget = "mysql"
	}

	// 1. Resolve backup file path if omitted
	if backupFilePath == "" {
		var err error
		backupFilePath, err = FindLatestDBBackup(backupDir, engine, dbName)
		if err != nil {
			return err
		}
	}

	if fi, err := os.Stat(backupFilePath); err != nil || fi.IsDir() {
		return fmt.Errorf("backup file not found: %s", backupFilePath)
	}

	// 2. Inspect manifest from backup file
	manifest, err := ReadDBManifestFromFile(backupFilePath)
	if err != nil {
		log.Printf("Warning: failed to read SQL header manifest: %v", err)
	}

	fmt.Println()
	fmt.Println("==============================================================================")
	fmt.Println("  WARNING: RESTORING DATABASE WILL OVERWRITE EXISTING DATA!")
	fmt.Println("==============================================================================")
	fmt.Printf("  Container Target : %s (%s)\n", containerTarget, engine)
	fmt.Printf("  Destination DB   : %s\n", dbName)
	fmt.Printf("  Backup File      : %s\n", backupFilePath)

	if engine == "redis" && dbName == "" {
		dbName = "redis"
	}

	if manifest != nil {
		fmt.Printf("  Backup Metadata  : Engine=%s, Database=%s, CreatedAt=%s\n", manifest.Engine, manifest.Database, manifest.CreatedAt)
		if manifest.Engine != "" && !strings.EqualFold(manifest.Engine, engine) {
			fmt.Printf("  [WARNING] Engine mismatch! Backup was created for %s, but restoring to %s!\n", manifest.Engine, engine)
		}
		if manifest.Database != "" && manifest.Database != dbName {
			fmt.Printf("  [WARNING] Database name mismatch! Backup is for %q, but restoring to %q!\n", manifest.Database, dbName)
		}
	} else {
		fmt.Println("  [NOTICE] No Oops metadata manifest found in backup file (migrated from external system).")
	}
	fmt.Println("==============================================================================")

	// 3. User Confirmation Guard
	if !force {
		confirmPrompt := dbName
		if confirmPrompt == "" {
			confirmPrompt = engine
		}
		fmt.Printf("Please type the target %q to proceed: ", confirmPrompt)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		trimmed := strings.TrimSpace(input)
		if trimmed != confirmPrompt {
			return fmt.Errorf("restore aborted: confirmation mismatch (got %q, expected %q)", trimmed, confirmPrompt)
		}
	}

	// 4. Execute streaming restore
	file, err := os.Open(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var sqlReader io.Reader = file
	if strings.HasSuffix(backupFilePath, ".gz") {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to decompress gzip stream: %w", err)
		}
		defer gzr.Close()
		sqlReader = gzr
	}

	log.Printf("==> Streaming restore into container %s for %s (%s)...", containerTarget, dbName, engine)

	var cmd *exec.Cmd
	if engine == "mysql" {
		rootPass := os.Getenv("MYSQL_ROOT_PASSWORD")
		var args []string
		if rootPass != "" {
			args = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", "-p" + rootPass, dbName}
		} else {
			args = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", dbName}
		}
		cmd = exec.CommandContext(ctx, "docker", args...)
	} else if engine == "pg" || engine == "postgres" {
		pgUser := os.Getenv("POSTGRES_USER")
		if pgUser == "" {
			pgUser = "postgres"
		}
		args := []string{"exec", "-i", containerTarget, "psql", "-U", pgUser, "-d", dbName}
		cmd = exec.CommandContext(ctx, "docker", args...)
	} else if engine == "redis" {
		// For Redis: stream decompressed RDB into /data/dump.rdb inside the container and restart
		cmd = exec.CommandContext(ctx, "docker", "exec", "-i", containerTarget, "sh", "-c", "cat > /data/dump.rdb")
		cmd.Stdin = sqlReader
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to write RDB snapshot to redis container: %w", err)
		}
		log.Printf("==> Restarting Redis container %s to load restored snapshot...", containerTarget)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", containerTarget)
		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart redis container: %w", err)
		}
		log.Printf("==> Successfully restored Redis snapshot from %s!", backupFilePath)
		return nil
	} else {
		return fmt.Errorf("unsupported database engine: %s", engine)
	}

	cmd.Stdin = sqlReader
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}

	log.Printf("==> Successfully restored database %s from %s!", dbName, backupFilePath)
	return nil
}

// ExecuteRestoreData restores a .tar.gz filesystem data archive
func ExecuteRestoreData(ctx context.Context, workDir string, backupDir string, targetOrFile string, dryRun bool, force bool) error {
	// 1. Resolve archive path
	archivePath := targetOrFile
	if _, err := os.Stat(archivePath); err != nil {
		// Try finding in backupDir
		candidate := filepath.Join(backupDir, targetOrFile)
		if _, err2 := os.Stat(candidate); err2 == nil {
			archivePath = candidate
		} else {
			// Try finding latest for name
			latest, err3 := FindLatestDataBackup(backupDir, targetOrFile)
			if err3 != nil {
				return fmt.Errorf("data backup file or target not found: %s", targetOrFile)
			}
			archivePath = latest
		}
	}

	// 2. Validate workspace root
	if !isWorkspaceValid(workDir) {
		return fmt.Errorf("refusing to restore: %s does not appear to be a valid Oopsbox workspace (missing stacks/ or compose.yml)", workDir)
	}

	// 3. Inspect manifest
	manifest, err := ReadDataManifestFromArchive(archivePath)
	if err != nil {
		log.Printf("Warning: error reading archive manifest: %v", err)
	}

	// 4. Preview contents
	files, totalBytes, err := inspectTarGz(archivePath)
	if err != nil {
		return fmt.Errorf("failed to inspect archive: %w", err)
	}

	fmt.Println()
	fmt.Println("==============================================================================")
	fmt.Println("  DATA RESTORE PREVIEW")
	fmt.Println("==============================================================================")
	fmt.Printf("  Workspace Root : %s\n", workDir)
	fmt.Printf("  Backup Archive : %s\n", archivePath)
	fmt.Printf("  Total Files    : %d files (%.2f MB)\n", len(files), float64(totalBytes)/(1024*1024))

	if manifest != nil {
		fmt.Printf("  Target Name    : %s\n", manifest.Name)
		fmt.Printf("  Created At     : %s\n", manifest.CreatedAt)
		fmt.Printf("  Source Paths   : %v\n", manifest.SourcePaths)

		// Cross-check with local oops.yml
		if cfg, err := docker.LoadOopsConfig(workDir); err == nil {
			foundInLocal := false
			for _, dt := range cfg.Backups.Data {
				if dt.Name == manifest.Name {
					foundInLocal = true
					break
				}
			}
			if !foundInLocal && len(cfg.Backups.Data) > 0 {
				fmt.Printf("  [NOTICE] Target %q not explicitly configured in destination oops.yml.\n", manifest.Name)
			}
		}
	} else {
		fmt.Println("  [NOTICE] No .oops-backup.json manifest found in archive (external archive).")
	}

	if dryRun {
		fmt.Println("\nFiles to be extracted:")
		for _, f := range files {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println("\n[Dry-run complete] No files were extracted.")
		return nil
	}

	fmt.Println("==============================================================================")

	// 5. User Confirmation
	if !force {
		fmt.Print("Type \"RESTORE\" to extract and overwrite existing files: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		trimmed := strings.TrimSpace(input)
		if trimmed != "RESTORE" {
			return fmt.Errorf("restore aborted: confirmation canceled")
		}
	}

	// 6. Extract archive securely
	log.Printf("==> Extracting %s into %s...", archivePath, workDir)
	if err := extractTarGz(archivePath, workDir); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	log.Printf("==> Successfully restored data from %s!", archivePath)
	return nil
}

func isWorkspaceValid(workDir string) bool {
	if fi, err := os.Stat(filepath.Join(workDir, "stacks")); err == nil && fi.IsDir() {
		return true
	}
	for _, c := range []string{"compose.yml", "compose.yaml", "oops.yml", "oops.yaml"} {
		if _, err := os.Stat(filepath.Join(workDir, c)); err == nil {
			return true
		}
	}
	return false
}

func inspectTarGz(archivePath string) ([]string, int64, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, 0, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var files []string
	var totalBytes int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		if hdr.Name == ManifestFileName {
			continue
		}
		files = append(files, hdr.Name)
		totalBytes += hdr.Size
	}

	return files, totalBytes, nil
}

func extractTarGz(archivePath string, workDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Name == ManifestFileName {
			continue
		}

		// Security: Prevent Zip Slip / Path Traversal
		cleanPath := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
			log.Printf("Warning: skipping illegal archive path: %s", hdr.Name)
			continue
		}

		targetPath := filepath.Join(workDir, cleanPath)

		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()
	}

	return nil
}

// FindLatestDBBackup finds the latest database backup file for the specified engine and DB
func FindLatestDBBackup(backupDir string, engine string, dbName string) (string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", fmt.Errorf("failed to read backup dir %s: %w", backupDir, err)
	}

	var matchPrefix string
	if dbName != "" {
		matchPrefix = fmt.Sprintf("%s_%s_", engine, dbName)
	} else {
		matchPrefix = fmt.Sprintf("%s_backup_", engine)
	}

	var latestFile string
	var latestTime int64

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, matchPrefix) && (strings.HasSuffix(name, ".sql.gz") || strings.HasSuffix(name, ".rdb.gz") || strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".rdb")) {
			info, err := e.Info()
			if err == nil && info.ModTime().Unix() > latestTime {
				latestTime = info.ModTime().Unix()
				latestFile = filepath.Join(backupDir, name)
			}
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no backup files found matching prefix %q in %s", matchPrefix, backupDir)
	}
	return latestFile, nil
}

// FindLatestDataBackup finds the latest data backup file matching the target name
func FindLatestDataBackup(backupDir string, targetName string) (string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", fmt.Errorf("failed to read backup dir %s: %w", backupDir, err)
	}

	matchPrefix := fmt.Sprintf("data_%s_", targetName)
	var latestFile string
	var latestTime int64

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, matchPrefix) && strings.HasSuffix(name, ".tar.gz") {
			info, err := e.Info()
			if err == nil && info.ModTime().Unix() > latestTime {
				latestTime = info.ModTime().Unix()
				latestFile = filepath.Join(backupDir, name)
			}
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no data backup files found matching prefix %q in %s", matchPrefix, backupDir)
	}
	return latestFile, nil
}
