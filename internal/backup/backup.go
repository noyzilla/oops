package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/noyzilla/oops/internal/docker"
)

// DumpMySQL executes mysqldump inside container and writes to gzip writer
func DumpMySQL(ctx context.Context, containerTarget string, dbName string, w io.Writer) error {
	WriteSQLHeader(w, DBManifest{
		Engine:    "mysql",
		Container: containerTarget,
		Database:  dbName,
	})

	rootPass := os.Getenv("MYSQL_ROOT_PASSWORD")
	var cmdArgs []string
	if dbName != "" {
		if rootPass != "" {
			cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "-p" + rootPass, "--databases", dbName}
		} else {
			cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "--databases", dbName}
		}
	} else {
		if rootPass != "" {
			cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "-p" + rootPass, "--all-databases"}
		} else {
			cmdArgs = []string{"exec", "-i", containerTarget, "mysqldump", "-u", "root", "--all-databases"}
		}
	}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DumpPostgres executes pg_dump or pg_dumpall inside container and writes to gzip writer
func DumpPostgres(ctx context.Context, containerTarget string, dbName string, w io.Writer) error {
	WriteSQLHeader(w, DBManifest{
		Engine:    "postgres",
		Container: containerTarget,
		Database:  dbName,
	})

	pgUser := os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		pgUser = "postgres"
	}

	var cmdArgs []string
	if dbName != "" {
		cmdArgs = []string{"exec", "-i", containerTarget, "pg_dump", "-U", pgUser, "-d", dbName}
	} else {
		cmdArgs = []string{"exec", "-i", containerTarget, "pg_dumpall", "-U", pgUser}
	}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DumpRedis saves and streams a point-in-time RDB snapshot from redis container
func DumpRedis(ctx context.Context, containerTarget string, w io.Writer) error {
	if containerTarget == "" {
		containerTarget = "redis"
	}
	// Best-effort SAVE inside container to ensure latest dirty data is persisted
	saveCmd := exec.CommandContext(ctx, "docker", "exec", containerTarget, "redis-cli", "SAVE")
	_ = saveCmd.Run()

	// Stream point-in-time RDB dump to writer
	cmd := exec.CommandContext(ctx, "docker", "exec", containerTarget, "redis-cli", "--rdb", "-")
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type dbBackupTask struct {
	Engine          string
	ContainerTarget string
	DatabaseName    string
	FileName        string
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
	var tasks []dbBackupTask

	if len(targets) == 0 {
		tasks = []dbBackupTask{
			{Engine: "mysql", ContainerTarget: "mysql", DatabaseName: "", FileName: fmt.Sprintf("mysql_backup_%s.sql.gz", timestamp)},
			{Engine: "postgres", ContainerTarget: "postgres", DatabaseName: "", FileName: fmt.Sprintf("postgres_backup_%s.sql.gz", timestamp)},
			{Engine: "redis", ContainerTarget: "redis", DatabaseName: "", FileName: fmt.Sprintf("redis_backup_%s.rdb.gz", timestamp)},
		}
	} else if len(targets) >= 2 {
		engineSpec := targets[0]
		dbName := targets[1]
		parts := strings.SplitN(engineSpec, ":", 2)
		engine := strings.ToLower(parts[0])
		containerTarget := engine
		if len(parts) == 2 {
			containerTarget = parts[1]
		}
		if engine == "pg" {
			engine = "postgres"
		}
		ext := "sql.gz"
		if engine == "redis" {
			ext = "rdb.gz"
		}
		tasks = []dbBackupTask{
			{Engine: engine, ContainerTarget: containerTarget, DatabaseName: dbName, FileName: fmt.Sprintf("%s_%s_%s.%s", engine, dbName, timestamp, ext)},
		}
	} else {
		t := targets[0]
		if strings.Contains(t, "/") {
			parts := strings.SplitN(t, "/", 2)
			engineSpec := parts[0]
			dbName := parts[1]
			eParts := strings.SplitN(engineSpec, ":", 2)
			engine := strings.ToLower(eParts[0])
			containerTarget := engine
			if len(eParts) == 2 {
				containerTarget = eParts[1]
			}
			if engine == "pg" {
				engine = "postgres"
			}
			ext := "sql.gz"
			if engine == "redis" {
				ext = "rdb.gz"
			}
			tasks = []dbBackupTask{
				{Engine: engine, ContainerTarget: containerTarget, DatabaseName: dbName, FileName: fmt.Sprintf("%s_%s_%s.%s", engine, dbName, timestamp, ext)},
			}
		} else {
			parts := strings.SplitN(t, ":", 2)
			engine := strings.ToLower(parts[0])
			containerTarget := engine
			if len(parts) == 2 {
				containerTarget = parts[1]
			}
			if engine == "pg" {
				engine = "postgres"
			}
			ext := "sql.gz"
			if engine == "redis" {
				ext = "rdb.gz"
			}
			tasks = []dbBackupTask{
				{Engine: engine, ContainerTarget: containerTarget, DatabaseName: "", FileName: fmt.Sprintf("%s_backup_%s.%s", engine, timestamp, ext)},
			}
		}
	}

	for _, task := range tasks {
		filePath := filepath.Join(backupDir, task.FileName)
		f, err := os.Create(filePath)
		if err != nil {
			log.Printf("Failed to create file %s: %v", filePath, err)
			continue
		}

		gzWriter := gzip.NewWriter(f)
		var dumpErr error

		if strings.Contains(task.Engine, "mysql") {
			if task.DatabaseName != "" {
				log.Printf("==> Creating MySQL backup for database %q: %s...", task.DatabaseName, task.FileName)
			} else {
				log.Printf("==> Creating MySQL full backup: %s...", task.FileName)
			}
			dumpErr = DumpMySQL(ctx, task.ContainerTarget, task.DatabaseName, gzWriter)
		} else if strings.Contains(task.Engine, "postgres") || strings.Contains(task.Engine, "pg") {
			if task.DatabaseName != "" {
				log.Printf("==> Creating Postgres backup for database %q: %s...", task.DatabaseName, task.FileName)
			} else {
				log.Printf("==> Creating Postgres full backup: %s...", task.FileName)
			}
			dumpErr = DumpPostgres(ctx, task.ContainerTarget, task.DatabaseName, gzWriter)
		} else if strings.Contains(task.Engine, "redis") {
			log.Printf("==> Creating Redis snapshot backup: %s...", task.FileName)
			dumpErr = DumpRedis(ctx, task.ContainerTarget, gzWriter)
		} else {
			log.Printf("Skipping unrecognized engine target: %s", task.Engine)
			gzWriter.Close()
			f.Close()
			os.Remove(filePath)
			continue
		}

		gzWriter.Close()
		f.Close()

		if dumpErr != nil {
			log.Printf("Error dumping %s: %v", task.Engine, dumpErr)
			os.Remove(filePath)
		} else {
			log.Printf("Successfully created database backup: %s", filePath)
		}
	}

	// Prune old db backups
	deleted, err := PruneDBBackups(backupDir, retention)
	if err != nil {
		log.Printf("Warning during db backup pruning: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("Pruned %d expired database backup archives: %v", len(deleted), deleted)
	}

	return nil
}

// ExecuteDataBackup archives configured data paths into compressed tarballs and prunes old archives
func ExecuteDataBackup(ctx context.Context, workDir string, backupDir string, targetNames []string, retention time.Duration) error {
	if backupDir == "" {
		backupDir = "./backups"
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	cfg, err := docker.LoadOopsConfig(workDir)
	if err != nil {
		return fmt.Errorf("failed to load oops configuration: %w", err)
	}

	dataTargets := cfg.Backups.Data
	if len(dataTargets) == 0 {
		// Default fallback if not configured in oops.yml: check data/ folder
		if fi, err := os.Stat(filepath.Join(workDir, "data")); err == nil && fi.IsDir() {
			dataTargets = []docker.DataBackupTarget{
				{Name: "data", Path: "data"},
			}
		} else {
			log.Printf("No data backup targets found in oops.yml and no data/ directory exists.")
			return nil
		}
	}

	// Filter by targetNames if provided
	if len(targetNames) > 0 {
		filterMap := make(map[string]bool)
		for _, name := range targetNames {
			filterMap[name] = true
		}
		var filtered []docker.DataBackupTarget
		for _, dt := range dataTargets {
			if filterMap[dt.Name] {
				filtered = append(filtered, dt)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no matching data backup targets found for: %v", targetNames)
		}
		dataTargets = filtered
	}

	timestamp := time.Now().Format("20060102_150405")

	for _, dt := range dataTargets {
		paths := dt.GetPaths()
		if len(paths) == 0 {
			continue
		}

		fileName := fmt.Sprintf("data_%s_%s.tar.gz", dt.Name, timestamp)
		filePath := filepath.Join(backupDir, fileName)

		log.Printf("==> Creating Data backup %s for paths %v...", fileName, paths)
		f, err := os.Create(filePath)
		if err != nil {
			log.Printf("Failed to create file %s: %v", filePath, err)
			continue
		}

		manifest := &DataManifest{
			Version:     "1.0",
			Type:        "data",
			Name:        dt.Name,
			CreatedAt:   time.Now().Format(time.RFC3339),
			OopsVersion: CurrentVersion,
			SourcePaths: paths,
		}

		tarErr := TarGzPathsWithManifest(workDir, paths, manifest, f)
		f.Close()

		if tarErr != nil {
			log.Printf("Error archiving %s: %v", dt.Name, tarErr)
			os.Remove(filePath)
		} else {
			log.Printf("Successfully created data backup: %s", filePath)
		}
	}

	// Prune old data backups
	deleted, err := PruneDataBackups(backupDir, retention)
	if err != nil {
		log.Printf("Warning during data backup pruning: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("Pruned %d expired data backup archives: %v", len(deleted), deleted)
	}

	return nil
}

// ExecuteFullBackup runs both database backups and data volume backups
func ExecuteFullBackup(ctx context.Context, workDir string, backupDir string, retention time.Duration) error {
	log.Printf("===> [Backup] Starting Full System Backup (Database + Data Volumes)...")

	// 1. Run DB backup
	if err := ExecuteBackup(ctx, backupDir, nil, retention); err != nil {
		log.Printf("Warning during database backup: %v", err)
	}

	// 2. Run Data backup
	if err := ExecuteDataBackup(ctx, workDir, backupDir, nil, retention); err != nil {
		log.Printf("Warning during data backup: %v", err)
	}

	log.Printf("===> [Backup] Full System Backup completed.")
	return nil
}

// TarGzPaths archives given relative paths in workDir to a gzip-compressed tar archive
func TarGzPaths(workDir string, paths []string, w io.Writer) error {
	return TarGzPathsWithManifest(workDir, paths, nil, w)
}

// TarGzPathsWithManifest archives given relative paths and embeds .oops-backup.json manifest into the tarball
func TarGzPathsWithManifest(workDir string, paths []string, manifest *DataManifest, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	if manifest != nil {
		manifestData, err := json.MarshalIndent(manifest, "", "  ")
		if err == nil {
			header := &tar.Header{
				Name:    ManifestFileName,
				Mode:    0644,
				Size:    int64(len(manifestData)),
				ModTime: time.Now(),
			}
			if err := tw.WriteHeader(header); err == nil {
				_, _ = tw.Write(manifestData)
			}
		}
	}

	for _, p := range paths {
		fullPath := filepath.Join(workDir, p)
		fi, err := os.Stat(fullPath)
		if err != nil {
			log.Printf("Warning: path %s does not exist, skipping", fullPath)
			continue
		}

		if !fi.IsDir() {
			relPath, err := filepath.Rel(workDir, fullPath)
			if err != nil {
				relPath = p
			}
			if err := addSingleFileToTar(tw, workDir, relPath, fullPath, fi); err != nil {
				log.Printf("Warning: failed to add file %s: %v", relPath, err)
			}
			continue
		}

		_ = filepath.Walk(fullPath, func(curPath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Skip socket or pipe files
			if info.Mode()&os.ModeSocket != 0 || info.Mode()&os.ModeNamedPipe != 0 {
				return nil
			}

			if info.Name() == ".DS_Store" {
				return nil
			}

			relPath, err := filepath.Rel(workDir, curPath)
			if err != nil {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return nil
			}

			header.Name = filepath.ToSlash(relPath)
			if info.IsDir() {
				header.Name += "/"
			}

			if err := tw.WriteHeader(header); err != nil {
				return nil
			}

			if !info.IsDir() {
				file, err := os.Open(curPath)
				if err != nil {
					return nil
				}
				defer file.Close()
				_, _ = io.Copy(tw, file)
			}
			return nil
		})
	}

	return nil
}

func addSingleFileToTar(tw *tar.Writer, workDir string, relPath string, fullPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(relPath)
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}
