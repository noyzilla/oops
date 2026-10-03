package cmd

import (
	"context"
	"os"

	"github.com/noyzilla/oops/internal/backup"
	"github.com/spf13/cobra"
)

func newDBBackupCmd() *cobra.Command {
	var retention string

	cmd := &cobra.Command{
		Use:     "db-backup [targets...]",
		Aliases: []string{"backup"},
		Short:   "Executes database dump and retention prune",
		Long:    "Executes compressed database backups for mysql, postgres, and redis containers, and prunes old archives.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if retention == "" {
				retention = os.Getenv("OOPS_BACKUP_RETENTION")
				if retention == "" {
					retention = os.Getenv("BACKUP_RETENTION_DAYS")
					if retention == "" {
						retention = "7d"
					}
				}
			}

			ret, err := backup.ParseRetention(retention)
			if err != nil {
				return err
			}

			backupDir := os.Getenv("OOPS_BACKUP_DIR")
			if backupDir == "" {
				backupDir = os.Getenv("BACKUP_DIR")
				if backupDir == "" {
					backupDir = "./backups"
				}
			}

			return backup.ExecuteBackup(context.Background(), backupDir, args, ret)
		},
	}

	cmd.Flags().StringVarP(&retention, "retention", "r", "", "Backup retention period (default: 7d or $OOPS_BACKUP_RETENTION)")

	return cmd
}
