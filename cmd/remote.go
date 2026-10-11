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

var (
	remoteAddPathFlag string
)

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manages remote server environments and overlay capsules",
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
		return "", fmt.Errorf("not inside a valid oopsbox workspace directory (must contain 'services/' or compose.yaml)")
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
		explicitFlag = strings.TrimPrefix(explicitFlag, "@")
		capsule := remote.CapsuleDir(workDir, explicitFlag)
		gitName := toGitRemoteName(explicitFlag)
		if _, err := os.Stat(capsule); err == nil || gitRemoteExists(workDir, gitName) || gitRemoteExists(workDir, explicitFlag) {
			return "", fmt.Errorf("remote '%s' already exists in this workspace", explicitFlag)
		}
		return explicitFlag, nil
	}

	// 1. Check if prod is available
	capsuleProd := remote.CapsuleDir(workDir, "prod")
	if _, err := os.Stat(capsuleProd); os.IsNotExist(err) && !gitRemoteExists(workDir, "oops-prod") && !gitRemoteExists(workDir, "prod") {
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

	capsuleHost := remote.CapsuleDir(workDir, host)
	if _, err := os.Stat(capsuleHost); os.IsNotExist(err) && !gitRemoteExists(workDir, toGitRemoteName(host)) && !gitRemoteExists(workDir, host) {
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

	fallbackCapsule := remote.CapsuleDir(workDir, fallback)
	if _, err := os.Stat(fallbackCapsule); err == nil || gitRemoteExists(workDir, toGitRemoteName(fallback)) || gitRemoteExists(workDir, fallback) {
		return "", fmt.Errorf("remote '%s' already exists in this workspace. Specify '@<name>' to choose a unique remote name", fallback)
	}

	return fallback, nil
}

func newRemoteAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <ssh-target> [@<name>] [--path <remote-path>]",
		Short: "Registers a remote server and provisions /@<name>/remote.yaml",
		Long:  "Creates local remote capsule /@<name>/ with remote.yaml manifest and verifies connectivity over SSH.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			sshTarget := ""
			name := ""
			for _, arg := range args {
				if strings.HasPrefix(arg, "@") {
					name = strings.TrimPrefix(arg, "@")
				} else if sshTarget == "" {
					sshTarget = arg
				} else if name == "" {
					name = arg
				}
			}

			if name != "" {
				name, err = deriveRemoteName(workDirAbs, sshTarget, name)
				if err != nil {
					return err
				}
			} else {
				name, err = deriveRemoteName(workDirAbs, sshTarget, "")
				if err != nil {
					return err
				}
			}
			name = strings.TrimPrefix(name, "@")

			remotePath := remoteAddPathFlag
			if remotePath == "" {
				remotePath = "~/oopsbox"
			}

			cmd.Printf("==> Registering remote server '@%s' (%s)...\n", name, sshTarget)

			// 1. Verify remote SSH connectivity & Docker availability
			pingCmd := "docker info >/dev/null 2>&1 || true"
			if out, err := remote.ExecuteRemoteSSH(sshTarget, pingCmd); err != nil {
				cmd.Printf("Notice: could not verify remote docker daemon on %s: %v (%s)\n", sshTarget, err, out)
			}

			// 2. Save remote.yaml in /@<name>/
			cfg := &remote.Config{
				Host: sshTarget,
				Path: remotePath,
			}
			if err := remote.SaveConfig(workDirAbs, name, cfg); err != nil {
				return err
			}

			cmd.Printf("\n✓ Successfully registered remote '@%s'\n", name)
			cmd.Printf("  Host             : %s\n", cfg.Host)
			cmd.Printf("  Remote Path      : %s\n", cfg.Path)
			cmd.Printf("  Manifest         : /@%s/remote.yaml\n", name)
			cmd.Println()
			cmd.Printf("To deploy your workspace to '@%s', run:\n", name)
			cmd.Printf("  oops deploy @%s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&remoteAddPathFlag, "path", "~/oopsbox", "Remote working directory path")
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
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			remotes, err := remote.ListRemotes(workDirAbs)
			if err != nil {
				return err
			}

			if len(remotes) == 0 {
				gitRemotes := listOopsGitRemoteDetails(workDirAbs)
				if len(gitRemotes) == 0 {
					cmd.Println("No remote servers registered for this workspace.")
					cmd.Println("Add a remote server using: oops remote add <ssh-target> [@<name>]")
					return nil
				}
				cmd.Println("Registered Remote Servers (Legacy Git Remotes):")
				for _, r := range gitRemotes {
					cmd.Printf("  %-15s (%s)\n", r.LogicalName, r.URL)
				}
				return nil
			}

			cmd.Println("Registered Remote Servers:")
			for _, rName := range remotes {
				cfg, err := remote.LoadConfig(workDirAbs, rName)
				if err != nil {
					cmd.Printf("  @%-14s (error: %v)\n", rName, err)
					continue
				}
				cmd.Printf("  @%-14s Host: %-25s Path: %s\n", rName, cfg.Host, cfg.Path)
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
		Short: "Removes a registered remote server capsule from current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimPrefix(args[0], "@")
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			capsuleErr := remote.RemoveRemote(workDirAbs, name)
			gitName := toGitRemoteName(name)
			gitErr := exec.Command("git", "-C", workDirAbs, "remote", "remove", gitName).Run()
			if gitErr != nil {
				_ = exec.Command("git", "-C", workDirAbs, "remote", "remove", name).Run()
			}

			if capsuleErr != nil && gitErr != nil {
				return fmt.Errorf("failed removing remote '@%s': %v", name, capsuleErr)
			}
			cmd.Printf("✓ Removed remote '@%s' from workspace\n", name)
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
			oldName := strings.TrimPrefix(args[0], "@")
			newName := strings.TrimPrefix(args[1], "@")
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			oldCapsule := remote.CapsuleDir(workDirAbs, oldName)
			newCapsule := remote.CapsuleDir(workDirAbs, newName)

			if _, err := os.Stat(oldCapsule); err == nil {
				if _, err := os.Stat(newCapsule); err == nil {
					return fmt.Errorf("remote '@%s' already exists", newName)
				}
				if err := os.Rename(oldCapsule, newCapsule); err != nil {
					return fmt.Errorf("failed renaming remote '@%s' to '@%s': %w", oldName, newName, err)
				}
				cmd.Printf("✓ Renamed remote '@%s' to '@%s'\n", oldName, newName)
				return nil
			}

			// Fallback to git remote rename
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
	candidateName := strings.TrimPrefix(candidate, "@")
	remotes, _ := remote.ListRemotes(workDir)
	for _, r := range remotes {
		if r == candidateName {
			return true
		}
	}
	gitRemotes := listGitRemotes(workDir)
	for _, r := range gitRemotes {
		if r == candidate || toLogicalRemoteName(r) == candidate {
			return true
		}
	}
	return false
}

func resolveDefaultServer(workDir string) string {
	remotes, _ := remote.ListRemotes(workDir)
	for _, r := range remotes {
		if r == "prod" {
			return "prod"
		}
	}
	if len(remotes) > 0 {
		return remotes[0]
	}

	remoteDetails := listOopsGitRemoteDetails(workDir)
	for _, r := range remoteDetails {
		if r.LogicalName == "prod" {
			return "prod"
		}
	}
	if len(remoteDetails) == 1 {
		return remoteDetails[0].LogicalName
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
	cleanName := strings.TrimPrefix(serverName, "@")
	cfg, err := remote.LoadConfig(workDir, cleanName)
	if err == nil && cfg.Host != "" {
		return cfg.Host
	}

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
