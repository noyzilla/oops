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
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
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

// GetActiveBox reads the active oopsbox path from ~/.oops/active_box if present.
func GetActiveBox() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", nil
	}
	activeFile := filepath.Join(home, ".oops", "active_box")
	data, err := os.ReadFile(activeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	path := strings.TrimSpace(string(data))
	if path == "" {
		return "", nil
	}
	return expandHome(path), nil
}

// SetActiveBox writes the active oopsbox path to ~/.oops/active_box.
func SetActiveBox(path string) error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	dir := filepath.Join(home, ".oops")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	activeFile := filepath.Join(dir, "active_box")
	return os.WriteFile(activeFile, []byte(strings.TrimSpace(path)+"\n"), 0644)
}

// CanonicalPath returns absolute and symlink-evaluated path for reliable comparison.
func CanonicalPath(path string) string {
	expanded := expandHome(path)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		abs = expanded
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return eval
	}
	return abs
}

// ValidateActiveBox verifies that workDir matches the active box recorded in ~/.oops/active_box.
// If ~/.oops/active_box does not exist, validation succeeds (single box / server mode).
func ValidateActiveBox(workDir string) error {
	activeBox, err := GetActiveBox()
	if err != nil || activeBox == "" {
		return nil
	}

	// If active box path on disk was removed, do not block
	if fi, err := os.Stat(activeBox); err != nil || !fi.IsDir() {
		return nil
	}

	workDirCanonical := CanonicalPath(workDir)
	activeBoxCanonical := CanonicalPath(activeBox)

	if workDirCanonical != activeBoxCanonical {
		return fmt.Errorf("active oopsbox mismatch!\n  Active Box : %s\n  Target Box : %s\n\nRunning containers belong to the active box. To switch active context, run:\n  oopsbox switch %s", activeBox, CanonicalPath(workDir), CanonicalPath(workDir))
	}

	return nil
}

// ResolveWorkDir returns the resolved working directory based on -C/--dir, current directory, ~/.oops/active_box, OOPSBOX_DIR, or well-known paths
func ResolveWorkDir(customDir string) string {
	// 1. Explicit -C / --dir flag (highest precedence)
	if customDir != "" {
		return expandHome(customDir)
	}

	// 2. Current directory if it is a valid oopsbox workspace
	if hasComposeContent(".") {
		return "."
	}

	// 3. Active box state (~/.oops/active_box)
	if activeBox, err := GetActiveBox(); err == nil && activeBox != "" {
		if hasComposeContent(activeBox) {
			return activeBox
		}
	}

	// 4. Environment variable (when outside an oopsbox directory and no active_box)
	if envDir := os.Getenv("OOPSBOX_DIR"); envDir != "" {
		return expandHome(envDir)
	}

	// 5. Check $HOME/oopsbox
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeOopsbox := filepath.Join(home, "oopsbox")
		if hasComposeContent(homeOopsbox) {
			return homeOopsbox
		}
	}

	// 6. Check /opt/oopsbox
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
	for _, c := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
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
	rootCmd.PersistentFlags().StringVarP(&targetDir, "dir", "C", "", "Target oopsbox working directory (default: OOPSBOX_DIR, ~/.oops/active_box, or auto-detect)")

	rootCmd.AddCommand(newServerCmd())
	rootCmd.AddCommand(newUpCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newDownCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newPullCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newSwitchCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newBackupCmd())
	rootCmd.AddCommand(newBackupDBCmd())
	rootCmd.AddCommand(newBackupDataCmd())
	rootCmd.AddCommand(newRestoreCmd())
	rootCmd.AddCommand(newRestoreDBCmd())
	rootCmd.AddCommand(newRestoreDataCmd())
	rootCmd.AddCommand(newDNSCmd())
}
