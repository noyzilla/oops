package cmd

import (
	"context"
	"os"
	"time"

	"github.com/noyzilla/oops/internal/backup"
	"github.com/spf13/cobra"
)

func newDBBackupCmd() *cobra.Command {
	var retention string

	cmd := &cobra.Command{
		Use:   "db-backup [targets...]",
		Short: "Executes database dump and retention prune",
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

	return cmd
}

func resolveBackupConfig(customRetention string) (retention time.Duration, backupDir string, err error) {
	if customRetention == "" {
		customRetention = os.Getenv("OOPS_BACKUP_RETENTION")
		if customRetention == "" {
			customRetention = os.Getenv("BACKUP_RETENTION_DAYS")
			if customRetention == "" {
				customRetention = "7d"
			}
		}
	}

	ret, err := backup.ParseRetention(customRetention)
	if err != nil {
		return 0, "", err
	}

	backupDir = os.Getenv("OOPS_BACKUP_DIR")
	if backupDir == "" {
		backupDir = os.Getenv("BACKUP_DIR")
		if backupDir == "" {
			backupDir = "./backups"
		}
	}

	return ret, backupDir, nil
}
