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
	"time"

	"github.com/noyzilla/oops/internal/db"
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
	} else if _, err := os.Stat(backupFilePath); err != nil {
		candidate := filepath.Join(backupDir, backupFilePath)
		if _, err2 := os.Stat(candidate); err2 == nil {
			backupFilePath = candidate
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
		rootPass := os.Getenv("OOPS_DB_MYSQL_PASSWORD")
		if rootPass == "" {
			rootPass = db.InspectContainerEnv(ctx, containerTarget, "MYSQL_ROOT_PASSWORD")
		}
		var args []string
		if rootPass != "" {
			args = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", "-p" + rootPass, dbName}
		} else {
			args = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", dbName}
		}
		cmd = exec.CommandContext(ctx, "docker", args...)
	} else if engine == "pg" || engine == "postgres" {
		pgUser := db.InspectContainerEnv(ctx, containerTarget, "POSTGRES_USER")
		if pgUser == "" {
			pgUser = os.Getenv("POSTGRES_USER")
		}
		if pgUser == "" {
			pgUser = "postgres"
		}
		pgPass := os.Getenv("OOPS_DB_POSTGRES_PASSWORD")
		if pgPass == "" {
			pgPass = db.InspectContainerEnv(ctx, containerTarget, "POSTGRES_PASSWORD")
		}
		var args []string
		if pgPass != "" {
			args = []string{"exec", "-i", "-e", "PGPASSWORD=" + pgPass, containerTarget, "psql", "-U", pgUser, "-d", dbName}
		} else {
			args = []string{"exec", "-i", containerTarget, "psql", "-U", pgUser, "-d", dbName}
		}
		cmd = exec.CommandContext(ctx, "docker", args...)
	} else if engine == "redis" {
		return RestoreRedis(ctx, containerTarget, sqlReader, backupFilePath)
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

// RestoreRedis safely restores an RDB snapshot into Redis, handling AOF transpilation
// and preserving safety backups of existing data.
func RestoreRedis(ctx context.Context, containerTarget string, rdbReader io.Reader, backupFilePath string) error {
	if containerTarget == "" {
		containerTarget = "redis"
	}

	// 1. Inspect container image
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.Config.Image}}", containerTarget)
	out, _ := inspectCmd.Output()
	redisImage := strings.TrimSpace(string(out))
	if redisImage == "" {
		redisImage = "redis:7-alpine"
	}

	// 2. Stop running main Redis container to freeze memory and prevent shutdown overwrite
	log.Printf("==> Stopping Redis container %s to prevent memory overwrite...", containerTarget)
	stopCmd := exec.CommandContext(ctx, "docker", "stop", containerTarget)
	_ = stopCmd.Run()

	// 3. Backup existing /data/appendonlydir and /data/dump.rdb, then stream new dump.rdb
	timestamp := time.Now().Format("20060102150405")
	prepScript := fmt.Sprintf(`
if [ -d /data/appendonlydir ]; then
    echo "==> Backing up existing appendonlydir to /data/appendonlydir.bak.%s"
    mv /data/appendonlydir /data/appendonlydir.bak.%s
fi
if [ -f /data/dump.rdb ]; then
    echo "==> Backing up existing dump.rdb to /data/dump.rdb.bak.%s"
    cp /data/dump.rdb /data/dump.rdb.bak.%s
fi
cat > /data/dump.rdb
chmod 660 /data/dump.rdb
`, timestamp, timestamp, timestamp, timestamp)

	prepCmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--volumes-from", containerTarget, redisImage, "sh", "-c", prepScript)
	prepCmd.Stdin = rdbReader
	prepCmd.Stdout = os.Stdout
	prepCmd.Stderr = os.Stderr
	if err := prepCmd.Run(); err != nil {
		_ = exec.CommandContext(ctx, "docker", "start", containerTarget).Run()
		return fmt.Errorf("failed to prepare and stream RDB snapshot to redis container: %w", err)
	}

	// 4. Transpile RDB -> AOF via ephemeral helper container running --appendonly no
	log.Printf("==> Transpiling RDB snapshot into Redis AOF structure...")
	converterName := fmt.Sprintf("oops-redis-converter-%s", timestamp)
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", converterName).Run()

	runCmd := exec.CommandContext(ctx, "docker", "run", "--name", converterName, "--volumes-from", containerTarget, "-d", redisImage,
		"redis-server", "--appendonly", "no", "--dir", "/data", "--dbfilename", "dump.rdb")
	if err := runCmd.Run(); err != nil {
		_ = exec.CommandContext(ctx, "docker", "start", containerTarget).Run()
		return fmt.Errorf("failed to run redis converter container: %w", err)
	}
	defer func() {
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", converterName).Run()
	}()

	// Wait for converter to be ready
	ready := false
	for i := 0; i < 30; i++ {
		pingCmd := exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "ping")
		if pingOut, pingErr := pingCmd.Output(); pingErr == nil && strings.Contains(string(pingOut), "PONG") {
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		_ = exec.CommandContext(ctx, "docker", "start", containerTarget).Run()
		return fmt.Errorf("redis converter container timed out waiting for ready")
	}

	// Log keys loaded into memory
	dbsizeCmd := exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "DBSIZE")
	if sizeOut, sizeErr := dbsizeCmd.Output(); sizeErr == nil {
		log.Printf("==> Loaded %s keys from snapshot into memory", strings.TrimSpace(string(sizeOut)))
	}

	// Enable appendonly live -> triggers background AOF rewrite
	configCmd := exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "CONFIG", "SET", "appendonly", "yes")
	if confOut, confErr := configCmd.CombinedOutput(); confErr != nil {
		_ = exec.CommandContext(ctx, "docker", "start", containerTarget).Run()
		return fmt.Errorf("failed to enable AOF on converter: %s: %w", string(confOut), confErr)
	}

	// Wait for AOF rewrite to complete (aof_rewrite_in_progress == 0)
	aofFinished := false
	for i := 0; i < 60; i++ {
		infoCmd := exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "INFO", "persistence")
		if infoOut, infoErr := infoCmd.Output(); infoErr == nil {
			infoStr := string(infoOut)
			if strings.Contains(infoStr, "aof_rewrite_in_progress:0") {
				aofFinished = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !aofFinished {
		log.Printf("Warning: AOF rewrite poll exceeded timeout, proceeding...")
	}

	// Save and shutdown converter
	_ = exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "save").Run()
	_ = exec.CommandContext(ctx, "docker", "exec", converterName, "redis-cli", "shutdown", "nosave").Run()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", converterName).Run()

	// 5. Start main Redis container
	log.Printf("==> Starting main Redis container %s with restored AOF...", containerTarget)
	startCmd := exec.CommandContext(ctx, "docker", "start", containerTarget)
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to restart main redis container: %w", err)
	}

	// 6. Log final keys in main container
	time.Sleep(1 * time.Second)
	pass := os.Getenv("OOPS_DB_REDIS_PASSWORD")
	if pass == "" {
		pass = db.InspectContainerEnv(ctx, containerTarget, "REDIS_PASSWORD")
	}
	cliArgs := []string{"exec", containerTarget, "redis-cli"}
	if pass != "" {
		cliArgs = append(cliArgs, "-a", pass, "--no-auth-warning")
	}
	cliArgs = append(cliArgs, "DBSIZE")
	dbsizeMainCmd := exec.CommandContext(ctx, "docker", cliArgs...)
	if out, err := dbsizeMainCmd.Output(); err == nil {
		log.Printf("==> Restored Redis database size: %s keys", strings.TrimSpace(string(out)))
	}

	log.Printf("==> Successfully restored Redis snapshot from %s!", backupFilePath)
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
		return fmt.Errorf("refusing to restore: %s does not appear to be a valid Oopsbox workspace (missing services/ or compose.yaml)", workDir)
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

		// Cross-check with local oops.yaml
		if cfg, err := docker.LoadOopsConfig(workDir); err == nil {
			foundInLocal := false
			for _, dt := range cfg.Backups.Data {
				if dt.Name == manifest.Name {
					foundInLocal = true
					break
				}
			}
			if !foundInLocal && len(cfg.Backups.Data) > 0 {
				fmt.Printf("  [NOTICE] Target %q not explicitly configured in destination oops.yaml.\n", manifest.Name)
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
	if fi, err := os.Stat(filepath.Join(workDir, "services")); err == nil && fi.IsDir() {
		return true
	}
	for _, c := range []string{"compose.yaml", "oops.yaml"} {
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
