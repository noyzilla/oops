package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var targetDir string

var rootCmd = &cobra.Command{
	Use:   "oops",
	Short: "Oops: Unified DevOps Orchestration & Container Platform",
	Long:  "Oops is a unified binary for managing multi-stack Docker Compose deployments and webhook automation.",
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

// ResolveWorkDir returns the resolved working directory based on -C/--dir, OOPS_DIR, or auto-discovery
func ResolveWorkDir(customDir string) string {
	if customDir != "" {
		return expandHome(customDir)
	}

	if envDir := os.Getenv("OOPS_DIR"); envDir != "" {
		return expandHome(envDir)
	}

	// Check if current directory has stacks/ or compose files
	if hasComposeContent(".") {
		return "."
	}

	// Check $HOME/oopsbox
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeOopsbox := filepath.Join(home, "oopsbox")
		if hasComposeContent(homeOopsbox) {
			return homeOopsbox
		}
	}

	// Check /opt/oopsbox
	optOopsbox := "/opt/oopsbox"
	if hasComposeContent(optOopsbox) {
		return optOopsbox
	}

	return "."
}

func hasComposeContent(dir string) bool {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return false
	}
	if fi, err := os.Stat(filepath.Join(dir, "stacks")); err == nil && fi.IsDir() {
		return true
	}
	for _, c := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return true
		}
	}
	return false
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&targetDir, "dir", "C", "", "Target oopsbox working directory (default: OOPS_DIR or auto-detect)")

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
