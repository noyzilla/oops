package cmd

import (
	"fmt"
	"os"

	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/noyzilla/oops/internal/storage"
	"github.com/spf13/cobra"
)

func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Inspect and link persistent storage paths",
	}
	cmd.AddCommand(newStorageCheckCmd(), newStorageLinkCmd())
	return cmd
}

func newStorageCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Reports dead links and unmounted paths used by containers and backups",
		Long:  "Inspects every bind-mount source of every stack plus the backup directory (OOPS_BACKUP_DIR). Exits non-zero when a dead link or a path on the OS disk under a mount prefix is found.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			backupDir := os.Getenv("OOPS_BACKUP_DIR")
			if backupDir == "" {
				backupDir = "./backups"
			}

			findings, err := orchestrator.InspectBox(workDir, backupDir)
			if err != nil {
				return err
			}
			if len(findings) == 0 {
				cmd.Println("Storage OK: no dead links or unmounted paths found.")
				return nil
			}
			for _, f := range findings {
				cmd.Printf("UNHEALTHY  %-24s %s\n           %s -> %s: %s\n", f.Stack, f.Source, f.Problem.Path, f.Problem.Target, f.Problem.Reason)
			}
			return fmt.Errorf("storage check found %d unhealthy path(s)", len(findings))
		},
	}
}

func newStorageLinkCmd() *cobra.Command {
	var name string
	var force bool

	cmd := &cobra.Command{
		Use:   "link <target>",
		Short: "Links the oopsbox data directory to a persistent disk path",
		Long:  "Creates <oopsbox>/data (or --name) as a symlink to <target> after verifying the target exists and is mounted. Refuses to replace a directory that contains data.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			linkPath, err := storage.NewChecker().Link(workDir, name, args[0], orchestrator.StoragePrefixes(workDir), force)
			if err != nil {
				return err
			}
			cmd.Printf("Linked %s -> %s\n", linkPath, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "data", "Link name inside the oopsbox")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing symlink")
	return cmd
}
