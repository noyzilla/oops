package cmd

import (
	"context"
	"os"
	"time"

	"github.com/noyzilla/oops/internal/backup"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

// newBackupCmd provides unified full system backup (DB + Data)
func newBackupCmd() *cobra.Command {
	var retention string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Executes full backup (database dumps + data volumes) and retention prune",
		Long:  "Executes full system backup for all database containers and filesystem data paths configured in oops.yml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			return backup.ExecuteFullBackup(context.Background(), workDir, backupDir, ret)
		},
	}

	cmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prunes all expired database and data backup archives based on retention policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			deleted, err := backup.PruneOldBackups(backupDir, ret)
			if err != nil {
				return err
			}
			cmd.Printf("Pruned %d expired backup archives.\n", len(deleted))
			for _, f := range deleted {
				cmd.Printf(" - %s\n", f)
			}
			return nil
		},
	}
	pruneCmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	cmd.AddCommand(pruneCmd)
	cmd.AddCommand(newBackupInspectCmd())
	cmd.AddCommand(newBackupDBCmd())
	cmd.AddCommand(newBackupDataCmd())

	return cmd
}

// newBackupDBCmd executes database dumps for mysql, postgres, and redis
func newBackupDBCmd() *cobra.Command {
	var retention string

	cmd := &cobra.Command{
		Use:   "backup-db [targets...]",
		Short: "Executes database dump and retention prune (mysql, postgres)",
		Long:  "Executes compressed database backups for mysql, postgres, and redis containers, and prunes old archives.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			return backup.ExecuteBackup(context.Background(), backupDir, args, ret)
		},
	}

	cmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prunes old database backup archives based on retention policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			deleted, err := backup.PruneDBBackups(backupDir, ret)
			if err != nil {
				return err
			}
			cmd.Printf("Pruned %d expired database backup archives.\n", len(deleted))
			for _, f := range deleted {
				cmd.Printf(" - %s\n", f)
			}
			return nil
		},
	}
	pruneCmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	cmd.AddCommand(pruneCmd)

	return cmd
}

// newBackupDataCmd executes filesystem data archives
func newBackupDataCmd() *cobra.Command {
	var retention string

	cmd := &cobra.Command{
		Use:   "backup-data [targets...]",
		Short: "Executes data volume and filesystem archives (uploads, storage) and retention prune",
		Long:  "Executes compressed tar archives for paths defined in oops.yml (backups.data), and prunes old archives.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			return backup.ExecuteDataBackup(context.Background(), workDir, backupDir, args, ret)
		},
	}

	cmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prunes old data backup archives based on retention policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			ret, backupDir, err := resolveBackupConfig(retention)
			if err != nil {
				return err
			}

			deleted, err := backup.PruneDataBackups(backupDir, ret)
			if err != nil {
				return err
			}
			cmd.Printf("Pruned %d expired data backup archives.\n", len(deleted))
			for _, f := range deleted {
				cmd.Printf(" - %s\n", f)
			}
			return nil
		},
	}
	pruneCmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	cmd.AddCommand(pruneCmd)

	return cmd
}

func resolveBackupConfig(customRetention string) (retention time.Duration, backupDir string, err error) {
	if customRetention == "" {
		customRetention = os.Getenv("OOPS_BACKUP_RETENTION")
		if customRetention == "" {
			customRetention = "7d"
		}
	}

	ret, err := backup.ParseRetention(customRetention)
	if err != nil {
		return 0, "", err
	}

	backupDir = os.Getenv("OOPS_BACKUP_DIR")
	if backupDir == "" {
		backupDir = "./backups"
	}

	// Backups are not mounted into compose, so they are validated here instead of by the container guard
	if err := orchestrator.CheckBackupDir(ResolveWorkDir(targetDir), backupDir); err != nil {
		return 0, "", err
	}

	return ret, backupDir, nil
}
