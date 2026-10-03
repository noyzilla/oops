package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oops",
	Short: "Oops: Unified DevOps Orchestration & Container Platform",
	Long:  "Oops is a unified binary for managing multi-layer Docker Compose stacks and webhook deployments.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// GetRootCommand returns the root cobra command for testing.
func GetRootCommand() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.AddCommand(newServerCmd())
	rootCmd.AddCommand(newUpCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newDownCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newPullCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newDBBackupCmd())
}
