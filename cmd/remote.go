package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/remote"
	"github.com/spf13/cobra"
)

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manages remote server environments and Git Bare synchronization",
		Long:  "Commands for registering remote servers, syncing/deploying workspace versions, and executing remote commands over SSH.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRemoteAddCmd())
	cmd.AddCommand(newRemoteListCmd())
	cmd.AddCommand(newRemoteRemoveCmd())

	return cmd
}

func RequireOopsboxWorkspace(customDir string) (string, error) {
	dir := customDir
	if dir == "" {
		dir = "."
	}
	expanded := expandHome(dir)
	if !hasComposeContent(expanded) {
		return "", fmt.Errorf("not inside a valid oopsbox workspace directory (must contain 'stacks/' or compose file)")
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return expanded, nil
	}
	return abs, nil
}

func parseRemoteAddArgs(args []string) (name, sshTarget string) {
	if len(args) == 1 {
		sshTarget = args[0]
		clean := strings.TrimPrefix(sshTarget, "ssh://")
		if idx := strings.Index(clean, "/"); idx != -1 {
			clean = clean[:idx]
		}
		if !strings.Contains(clean, "@") && !strings.Contains(clean, ":") {
			name = clean
		} else {
			name = "oopsbox"
		}
	} else if len(args) >= 2 {
		name = args[0]
		sshTarget = args[1]
	}
	return name, sshTarget
}

func newRemoteAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [name] <ssh-target>",
		Short: "Registers and bootstraps a remote server via SSH",
		Long:  "Connects to remote server via SSH, verifies/installs oops & docker compose, initializes bare repo at ~/.oops/oopsbox.git, and sets up local Git remote.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			name, sshTarget := parseRemoteAddArgs(args)
			bareRepoPath := "~/.oops/oopsbox.git"
			remoteBoxPath := "~/oopsbox"

			cmd.Printf("==> Bootstrapping remote server '%s' (%s)...\n", name, sshTarget)

			// 1. Generate remote script and execute via SSH
			script := remote.GenerateBootstrapRemoteScript(bareRepoPath, remoteBoxPath)
			out, err := remote.ExecuteRemoteSSH(sshTarget, script)
			if err != nil {
				cmd.Printf("Output: %s\n", out)
				return fmt.Errorf("remote bootstrapping failed: %w", err)
			}
			if strings.TrimSpace(out) != "" {
				cmd.Println(out)
			}

			// 2. Configure local Git remote
			cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
			if idx := strings.Index(cleanTarget, "/"); idx != -1 {
				cleanTarget = cleanTarget[:idx]
			}
			remoteURL := fmt.Sprintf("%s:.oops/oopsbox.git", cleanTarget)
			gitRemoteCmd := exec.Command("git", "-C", workDirAbs, "remote", "add", name, remoteURL)
			if err := gitRemoteCmd.Run(); err != nil {
				// If remote exists, update URL
				_ = exec.Command("git", "-C", workDirAbs, "remote", "set-url", name, remoteURL).Run()
			}

			cmd.Printf("\n✓ Successfully registered remote '%s' (%s)\n", name, remoteURL)
			cmd.Printf("  Bare Repository : %s\n", bareRepoPath)
			cmd.Printf("  Workspace Directory: %s\n", remoteBoxPath)
			cmd.Println()
			cmd.Printf("To deploy your workspace to '%s', run:\n", name)
			cmd.Printf("  oops deploy %s\n", name)
			return nil
		},
	}
}

func ResolveGitWorkDir(customDir string) string {
	if customDir != "" {
		return box.CanonicalPath(customDir)
	}
	if _, err := os.Stat(".git"); err == nil {
		return "."
	}
	return ResolveWorkDir("")
}

func newRemoteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists registered remote servers for current workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			gitCmd := exec.Command("git", "-C", workDirAbs, "remote", "-v")
			out, err := gitCmd.Output()
			if err != nil || len(out) == 0 {
				fmt.Println("No remote servers registered for this workspace.")
				fmt.Println("Add a remote server using: oops remote add [name] <ssh-target>")
				return nil
			}

			fmt.Println("Registered Remote Servers:")
			fmt.Print(string(out))
			return nil
		},
	}
}

func newRemoteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Removes a registered remote server configuration from current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			gitCmd := exec.Command("git", "-C", workDirAbs, "remote", "remove", name)
			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("failed removing remote '%s': %w", name, err)
			}
			fmt.Printf("✓ Removed remote '%s' from workspace\n", name)
			return nil
		},
	}
}

func isKnownRemoteOrHost(workDir, candidate string) bool {
	if candidate == "" {
		return false
	}
	if strings.Contains(candidate, "@") || strings.Contains(candidate, ":") || strings.HasPrefix(candidate, "ssh://") {
		return true
	}
	remotes := listGitRemotes(workDir)
	for _, r := range remotes {
		if r == candidate {
			return true
		}
	}
	return false
}

func resolveDefaultServer(workDir string) string {
	remotes := listGitRemotes(workDir)
	for _, r := range remotes {
		if r == "oopsbox" {
			return "oopsbox"
		}
	}
	if len(remotes) == 1 {
		return remotes[0]
	}
	return "oopsbox"
}

func listGitRemotes(workDir string) []string {
	cmd := exec.Command("git", "-C", workDir, "remote")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func resolveSSHTarget(workDir, serverName string) string {
	remoteURLCmd := exec.Command("git", "-C", workDir, "remote", "get-url", serverName)
	out, err := remoteURLCmd.Output()
	if err == nil && len(out) > 0 {
		return extractSSHTarget(string(out))
	}
	return extractSSHTarget(serverName)
}

func extractSSHTarget(remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if strings.HasPrefix(remoteURL, "ssh://") {
		remoteURL = strings.TrimPrefix(remoteURL, "ssh://")
		parts := strings.Split(remoteURL, "/")
		return parts[0]
	}
	if idx := strings.Index(remoteURL, ":"); idx != -1 {
		return remoteURL[:idx]
	}
	return remoteURL
}

func init() {
	_ = bytes.Buffer{}
}
