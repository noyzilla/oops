package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/noyzilla/oops/internal/backup"
	"github.com/spf13/cobra"
)

// newRestoreCmd provides root restore command and subcommands
func newRestoreCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restores database dumps and filesystem data archives",
		Long:  "Restores database tables and filesystem volumes with validation and confirmation guards.",
		RunE: func(cmd *cobra.Command, args []string) error {
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
		Use:   "restore-db <engine>[:target] <db_name> [backup_file]",
		Short: "Restores database dump into container with confirmation guard",
		Long:    "Restores .sql or .sql.gz database dump into a running database container (mysql, postgres).",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineSpec := args[0]
			dbName := args[1]
			var backupFile string
			if len(args) >= 3 {
				backupFile = args[2]
			}

			_, backupDir, err := resolveBackupConfig("")
			if err != nil {
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
		Long:    "Extracts compressed tar archive into workspace root based on manifest or relative paths.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetOrFile := args[0]

			_, backupDir, err := resolveBackupConfig("")
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
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
