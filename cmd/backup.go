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
			ret, err := backup.ParseRetention(retention)
			if err != nil {
				return err
			}

			backupDir := os.Getenv("BACKUP_DIR")
			if backupDir == "" {
				backupDir = "./backups"
			}

			return backup.ExecuteBackup(context.Background(), backupDir, args, ret)
		},
	}

	cmd.Flags().StringVarP(&retention, "retention", "r", "7d", "Backup retention period (e.g. 7d, 14d, 30d)")

	return cmd
}
