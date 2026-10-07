package cmd

import (
	"bufio"
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

var remoteAddNameFlag string

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
	cmd.AddCommand(newRemoteRenameCmd())

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

func RequireGitOopsboxWorkspace(customDir string) (string, error) {
	workDirAbs, err := RequireOopsboxWorkspace(customDir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "-C", workDirAbs, "rev-parse", "--show-toplevel")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("not inside a valid git project workspace (must be inside a git project repository with .git)")
	}
	return workDirAbs, nil
}

func toGitRemoteName(logicalName string) string {
	if strings.HasPrefix(logicalName, "oops-") {
		return logicalName
	}
	return "oops-" + logicalName
}

func toLogicalRemoteName(gitRemoteName string) string {
	if strings.HasPrefix(gitRemoteName, "oops-") {
		return strings.TrimPrefix(gitRemoteName, "oops-")
	}
	return gitRemoteName
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func gitRemoteExists(workDir, gitRemoteName string) bool {
	err := exec.Command("git", "-C", workDir, "remote", "get-url", gitRemoteName).Run()
	return err == nil
}

func deriveRemoteName(workDir, sshTarget, explicitFlag string) (string, error) {
	if explicitFlag != "" {
		gitName := toGitRemoteName(explicitFlag)
		if gitRemoteExists(workDir, gitName) || gitRemoteExists(workDir, explicitFlag) {
			return "", fmt.Errorf("remote '%s' already exists in this workspace", explicitFlag)
		}
		return explicitFlag, nil
	}

	// 1. Check if prod is available
	if !gitRemoteExists(workDir, "oops-prod") && !gitRemoteExists(workDir, "prod") {
		return "prod", nil
	}

	// 2. Extract host and user from sshTarget
	cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
	if idx := strings.Index(cleanTarget, "/"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}

	var host, user string
	if idx := strings.Index(cleanTarget, "@"); idx != -1 {
		user = cleanTarget[:idx]
		host = cleanTarget[idx+1:]
	} else {
		host = cleanTarget
	}
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if host == "" {
		host = "remote"
	}

	if !gitRemoteExists(workDir, toGitRemoteName(host)) && !gitRemoteExists(workDir, host) {
		return host, nil
	}

	// 3. Fallback to host_user
	fallback := host
	if user != "" {
		fallback = fmt.Sprintf("%s_%s", host, user)
	} else {
		fallback = fmt.Sprintf("%s_2", host)
	}

	if isTerminal() {
		fmt.Printf("Warning: remote name '%s' already exists.\n", host)
		fmt.Printf("Enter remote name [%s]: ", fallback)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				fallback = input
			}
		}
	}

	if gitRemoteExists(workDir, toGitRemoteName(fallback)) || gitRemoteExists(workDir, fallback) {
		return "", fmt.Errorf("remote '%s' already exists in this workspace. Use -r <name> to specify a unique remote name", fallback)
	}

	return fallback, nil
}

func newRemoteAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <ssh-target> [-r <remote-name>]",
		Short: "Registers and bootstraps a remote server via SSH",
		Long:  "Connects to remote server via SSH, verifies/installs oops & docker compose, initializes bare repo at ~/.oops/oopsbox.git, and sets up local Git remote.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireGitOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			sshTarget := args[0]
			name, err := deriveRemoteName(workDirAbs, sshTarget, remoteAddNameFlag)
			if err != nil {
				return err
			}

			gitRemoteName := toGitRemoteName(name)
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

			// 2. Configure local Git remote with oops- prefix
			cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
			if idx := strings.Index(cleanTarget, "/"); idx != -1 {
				cleanTarget = cleanTarget[:idx]
			}
			remoteURL := fmt.Sprintf("%s:.oops/oopsbox.git", cleanTarget)
			gitRemoteCmd := exec.Command("git", "-C", workDirAbs, "remote", "add", gitRemoteName, remoteURL)
			if err := gitRemoteCmd.Run(); err != nil {
				// If remote exists, update URL
				_ = exec.Command("git", "-C", workDirAbs, "remote", "set-url", gitRemoteName, remoteURL).Run()
			}

			cmd.Printf("\n✓ Successfully registered remote '%s' (%s)\n", name, remoteURL)
			cmd.Printf("  Bare Repository : %s\n", bareRepoPath)
			cmd.Printf("  Workspace Directory: %s\n", remoteBoxPath)
			cmd.Println()
			cmd.Printf("To deploy your workspace to '%s', run:\n", name)
			cmd.Printf("  oops deploy -r %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVarP(&remoteAddNameFlag, "remote", "r", "", "Specify logical remote server name")
	return cmd
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
			workDirAbs, err := RequireGitOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			remotes := listOopsGitRemoteDetails(workDirAbs)
			if len(remotes) == 0 {
				fmt.Println("No remote servers registered for this workspace.")
				fmt.Println("Add a remote server using: oops remote add <ssh-target> [-r <name>]")
				return nil
			}

			fmt.Println("Registered Remote Servers:")
			for _, r := range remotes {
				fmt.Printf("  %-15s (%s)\n", r.LogicalName, r.URL)
			}
			return nil
		},
	}
}

type RemoteDetail struct {
	LogicalName string
	GitName     string
	URL         string
}

func listOopsGitRemoteDetails(workDir string) []RemoteDetail {
	cmd := exec.Command("git", "-C", workDir, "remote", "-v")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	seen := make(map[string]bool)
	var result []RemoteDetail

	for _, l := range lines {
		fields := strings.Fields(l)
		if len(fields) >= 2 {
			gitName := fields[0]
			url := fields[1]

			if strings.HasSuffix(fields[len(fields)-1], "(fetch)") {
				if strings.HasPrefix(gitName, "oops-") || gitName == "oopsbox" || gitName == "prod" {
					logical := toLogicalRemoteName(gitName)
					if !seen[logical] {
						seen[logical] = true
						result = append(result, RemoteDetail{
							LogicalName: logical,
							GitName:     gitName,
							URL:         url,
						})
					}
				}
			}
		}
	}
	return result
}

func newRemoteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Removes a registered remote server configuration from current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			workDirAbs, err := RequireGitOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			gitName := toGitRemoteName(name)
			if err := exec.Command("git", "-C", workDirAbs, "remote", "remove", gitName).Run(); err != nil {
				// Fallback to un-prefixed name
				if errLegacy := exec.Command("git", "-C", workDirAbs, "remote", "remove", name).Run(); errLegacy != nil {
					return fmt.Errorf("failed removing remote '%s': %w", name, err)
				}
			}
			fmt.Printf("✓ Removed remote '%s' from workspace\n", name)
			return nil
		},
	}
}

func newRemoteRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Renames a registered remote server configuration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]
			workDirAbs, err := RequireGitOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			oldGit := toGitRemoteName(oldName)
			newGit := toGitRemoteName(newName)

			if !gitRemoteExists(workDirAbs, oldGit) {
				if gitRemoteExists(workDirAbs, oldName) {
					oldGit = oldName
				} else {
					return fmt.Errorf("remote '%s' not found", oldName)
				}
			}

			if gitRemoteExists(workDirAbs, newGit) || gitRemoteExists(workDirAbs, newName) {
				return fmt.Errorf("remote '%s' already exists", newName)
			}

			gitCmd := exec.Command("git", "-C", workDirAbs, "remote", "rename", oldGit, newGit)
			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("failed renaming remote '%s' to '%s': %w", oldName, newName, err)
			}
			cmd.Printf("✓ Renamed remote '%s' to '%s'\n", oldName, newName)
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
		if r == candidate || toLogicalRemoteName(r) == candidate {
			return true
		}
	}
	return false
}

func resolveDefaultServer(workDir string) string {
	remotes := listOopsGitRemoteDetails(workDir)
	for _, r := range remotes {
		if r.LogicalName == "prod" {
			return "prod"
		}
	}
	if len(remotes) == 1 {
		return remotes[0].LogicalName
	}
	return "prod"
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
	gitRemoteName := toGitRemoteName(serverName)
	remoteURLCmd := exec.Command("git", "-C", workDir, "remote", "get-url", gitRemoteName)
	out, err := remoteURLCmd.Output()
	if err != nil {
		remoteURLCmd = exec.Command("git", "-C", workDir, "remote", "get-url", serverName)
		out, err = remoteURLCmd.Output()
	}
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
