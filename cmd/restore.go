package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/backup"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

// newRestoreCmd provides root restore command and subcommands
func newRestoreCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore [target_or_file] [backup_file]",
		Short: "Restores database dumps and filesystem data archives",
		Long:  "Restores database tables and filesystem volumes with validation and confirmation guards. Directly supports engines (redis, mysql, postgres), dump files (dump.rdb, .rdb, .sql, .tar.gz), or subcommands (restore-db, restore-data).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			workDir, _, backupDir, err := resolveBackupConfig("")
			if err != nil {
				return err
			}
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			target := args[0]
			targetLower := strings.ToLower(target)

			// 1. Direct Redis engine shortcut: oops restore redis [backup_file]
			if targetLower == "redis" || strings.HasPrefix(targetLower, "redis:") {
				var backupFile string
				if len(args) >= 2 {
					backupFile = args[1]
				}
				return backup.ExecuteRestoreDB(context.Background(), backupDir, target, "redis", backupFile, force)
			}

			// 2. Direct Redis RDB file: oops restore dump.rdb or *.rdb / *.rdb.gz
			if targetLower == "dump.rdb" || strings.HasSuffix(targetLower, ".rdb") || strings.HasSuffix(targetLower, ".rdb.gz") {
				return backup.ExecuteRestoreDB(context.Background(), backupDir, "redis", "redis", target, force)
			}

			// 3. Filesystem archive: oops restore <name>.tar.gz
			if strings.HasSuffix(targetLower, ".tar.gz") {
				return backup.ExecuteRestoreData(context.Background(), workDir, backupDir, target, false, force)
			}

			// 4. SQL archive: inspect manifest if available
			if strings.HasSuffix(targetLower, ".sql") || strings.HasSuffix(targetLower, ".sql.gz") {
				filePath := target
				if _, err := os.Stat(filePath); err != nil {
					candidate := filepath.Join(backupDir, filePath)
					if _, err2 := os.Stat(candidate); err2 == nil {
						filePath = candidate
					}
				}
				m, err := backup.ReadDBManifestFromFile(filePath)
				if err == nil && m != nil && m.Engine != "" {
					return backup.ExecuteRestoreDB(context.Background(), backupDir, m.Engine, m.Database, target, force)
				}
			}

			return cmd.Help()
		},
	}

	cmd.PersistentFlags().BoolVarP(&force, "yes", "y", false, "Bypass confirmation prompt (useful for non-interactive scripts)")

	cmd.AddCommand(newRestoreDBCmd())
	cmd.AddCommand(newRestoreDataCmd())

	return cmd
}

// newRestoreDBCmd restores database dump with validation and name confirmation
func newRestoreDBCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore-db <engine>[:target] [db_name] [backup_file]",
		Short: "Restores database dump into container with confirmation guard",
		Long:  "Restores .sql, .sql.gz, or .rdb database dump into a running database container (mysql, postgres, redis).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineSpec := args[0]
			parts := strings.SplitN(engineSpec, ":", 2)
			engine := strings.ToLower(parts[0])

			var dbName string
			var backupFile string

			if engine == "redis" {
				dbName = "redis"
				if len(args) == 2 {
					arg := args[1]
					if arg == "dump.rdb" || strings.HasSuffix(arg, ".rdb") || strings.HasSuffix(arg, ".rdb.gz") {
						backupFile = arg
					} else if _, err := os.Stat(arg); err == nil {
						backupFile = arg
					} else {
						dbName = arg
					}
				} else if len(args) >= 3 {
					dbName = args[1]
					backupFile = args[2]
				}
			} else {
				if len(args) < 2 {
					return fmt.Errorf("usage: oops restore-db %s <db_name> [backup_file]", engineSpec)
				}
				dbName = args[1]
				if len(args) >= 3 {
					backupFile = args[2]
				}
			}

			workDir, _, backupDir, err := resolveBackupConfig("")
			if err != nil {
				return err
			}
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			return backup.ExecuteRestoreDB(context.Background(), backupDir, engineSpec, dbName, backupFile, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "yes", "y", false, "Bypass database name re-type confirmation prompt")

	return cmd
}

// newRestoreDataCmd restores filesystem data volume archive
func newRestoreDataCmd() *cobra.Command {
	var force bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "restore-data <target_name|backup_file>",
		Short: "Restores filesystem data volume archive with dry-run preview and confirmation",
		Long:  "Extracts compressed tar archive into workspace root based on manifest or relative paths.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetOrFile := args[0]

			workDir, _, backupDir, err := resolveBackupConfig("")
			if err != nil {
				return err
			}
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			return backup.ExecuteRestoreData(context.Background(), workDir, backupDir, targetOrFile, dryRun, force)
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Preview files to be extracted without modifying files")
	cmd.Flags().BoolVarP(&force, "yes", "y", false, "Bypass confirmation prompt")

	return cmd
}

// newBackupInspectCmd inspects metadata manifest inside backup archives
func newBackupInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <backup_file>",
		Short: "Inspects metadata manifest in a database dump or data archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			if _, err := os.Stat(filePath); err != nil {
				workDir := ResolveWorkDir(targetDir)
				cfg, _ := docker.LoadOopsConfig(workDir)
				backupDir := resolveBackupDirForWorkDir(workDir, cfg)
				if _, err2 := os.Stat(filepath.Join(backupDir, filePath)); err2 == nil {
					filePath = filepath.Join(backupDir, filePath)
				} else if _, err3 := os.Stat(filepath.Join(workDir, filePath)); err3 == nil {
					filePath = filepath.Join(workDir, filePath)
				}
			}

			if strings.HasSuffix(filePath, ".tar.gz") {
				m, err := backup.ReadDataManifestFromArchive(filePath)
				if err != nil {
					return fmt.Errorf("failed to read data archive manifest: %w", err)
				}
				if m == nil {
					cmd.Println("[Notice] No .oops-backup.json manifest found in archive.")
					return nil
				}
				cmd.Println("==============================================================================")
				cmd.Println("  Data Archive Manifest")
				cmd.Println("==============================================================================")
				cmd.Printf("  Name         : %s\n", m.Name)
				cmd.Printf("  Type         : %s\n", m.Type)
				cmd.Printf("  Created At   : %s\n", m.CreatedAt)
				cmd.Printf("  Oops Version : %s\n", m.OopsVersion)
				cmd.Printf("  Source Paths : %v\n", m.SourcePaths)
				cmd.Println("==============================================================================")
				return nil
			}

			// SQL dump
			m, err := backup.ReadDBManifestFromFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read SQL dump manifest: %w", err)
			}
			if m == nil {
				cmd.Println("[Notice] No Oops database manifest found in SQL dump.")
				return nil
			}
			cmd.Println("==============================================================================")
			cmd.Println("  Database Backup Manifest")
			cmd.Println("==============================================================================")
			cmd.Printf("  Engine       : %s\n", m.Engine)
			cmd.Printf("  Container    : %s\n", m.Container)
			cmd.Printf("  Database     : %s\n", m.Database)
			cmd.Printf("  Created At   : %s\n", m.CreatedAt)
			cmd.Printf("  Oops Version : %s\n", m.OopsVersion)
			cmd.Println("==============================================================================")
			return nil
		},
	}
}
