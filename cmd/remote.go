package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/key"
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

func newRemoteAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <ssh-target> [remote-path]",
		Short: "Registers and bootstraps a remote server via SSH",
		Long:  "Connects to remote server via SSH, verifies/installs oops & docker compose, initializes bare repo, and sets up local Git remote.",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			sshTarget := args[1]
			remoteBoxPath := "~/oopsbox"
			if len(args) > 2 {
				remoteBoxPath = args[2]
			}

			bareRepoPath := fmt.Sprintf("~/.oops/repos/%s.git", name)
			workDir := ResolveGitWorkDir(targetDir)
			workDirAbs, err := filepath.Abs(workDir)
			if err != nil {
				workDirAbs = workDir
			}

			cmd.Printf("==> Bootstrapping remote server '%s' (%s)...\n", name, sshTarget)

			// 1. Generate remote script and execute via SSH
			script := remote.GenerateBootstrapRemoteScript(bareRepoPath, remoteBoxPath)
			out, err := remote.ExecuteRemoteSSH(sshTarget, script)
			if err != nil {
				cmd.Printf("Output: %s\n", out)
				return fmt.Errorf("remote bootstrapping failed: %w", err)
			}

			// 2. Configure local Git remote
			cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
			if idx := strings.Index(cleanTarget, "/"); idx != -1 {
				cleanTarget = cleanTarget[:idx]
			}
			remoteURL := fmt.Sprintf("%s:.oops/repos/%s.git", cleanTarget, name)
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
			cmd.Printf("  oops remote %s deploy\n", name)
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
			workDir := ResolveGitWorkDir(targetDir)
			gitCmd := exec.Command("git", "-C", workDir, "remote", "-v")
			out, err := gitCmd.Output()
			if err != nil || len(out) == 0 {
				fmt.Println("No remote servers registered for this workspace.")
				fmt.Println("Add a remote server using: oops remote add <name> <ssh-target> [remote-path]")
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
			workDir := ResolveGitWorkDir(targetDir)
			gitCmd := exec.Command("git", "-C", workDir, "remote", "remove", name)
			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("failed removing remote '%s': %w", name, err)
			}
			fmt.Printf("✓ Removed remote '%s' from workspace\n", name)
			return nil
		},
	}
}

// HandleDynamicRemoteCommands handles dynamic invocations like `oops remote <server> <cmd>`
func HandleDynamicRemoteCommands(args []string) (bool, error) {
	if len(args) < 2 || args[0] != "remote" {
		return false, nil
	}

	subCmd := args[1]
	// If standard subcommand, let Cobra handle it
	if subCmd == "add" || subCmd == "list" || subCmd == "remove" || subCmd == "--help" || subCmd == "-h" {
		return false, nil
	}

	serverName := subCmd
	action := "deploy"
	if len(args) > 2 {
		action = args[2]
	}

	workDir := ResolveGitWorkDir(targetDir)
	keyPath, _ := key.DefaultKeyPath()
	sshCmdStr := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new", keyPath)

	switch action {
	case "push":
		branch := "main"
		for i, a := range args {
			if (a == "-b" || a == "--branch") && i+1 < len(args) {
				branch = args[i+1]
			}
		}
		log.Printf("==> [Remote Sync] Pushing workspace to remote '%s' (%s)...", serverName, branch)
		gitCmd := exec.Command("git", "-C", workDir, "push", serverName, branch)
		gitCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmdStr)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return true, gitCmd.Run()

	case "deploy":
		targetRef := "main"
		for i, a := range args {
			if (a == "-b" || a == "--branch") && i+1 < len(args) {
				targetRef = args[i+1]
			}
			if (a == "-t" || a == "--tag") && i+1 < len(args) {
				targetRef = "refs/tags/" + args[i+1]
			}
		}
		log.Printf("==> [Remote Deploy] Deploying workspace to remote '%s' (%s)...", serverName, targetRef)
		gitCmd := exec.Command("git", "-C", workDir, "push", "-o", "deploy", serverName, targetRef)
		gitCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmdStr)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return true, gitCmd.Run()

	case "pull":
		log.Printf("==> [Remote Pull] Pulling updates from remote '%s'...", serverName)
		gitCmd := exec.Command("git", "-C", workDir, "pull", serverName, "main")
		gitCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmdStr)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return true, gitCmd.Run()

	case "up", "down", "ps", "logs":
		// Remote SSH delegation
		remoteURLCmd := exec.Command("git", "-C", workDir, "remote", "get-url", serverName)
		out, err := remoteURLCmd.Output()
		if err != nil {
			return true, fmt.Errorf("remote '%s' not found: %w", serverName, err)
		}
		sshTarget := extractSSHTarget(string(out))

		remoteCmd := fmt.Sprintf("oops %s", strings.Join(args[2:], " "))
		sshArgs := []string{
			"-i", keyPath,
			"-o", "StrictHostKeyChecking=accept-new",
			sshTarget,
			remoteCmd,
		}
		cmd := exec.Command("ssh", sshArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return true, cmd.Run()
	}

	return false, nil
}

func extractSSHTarget(remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if strings.HasPrefix(remoteURL, "ssh://") {
		remoteURL = strings.TrimPrefix(remoteURL, "ssh://")
		parts := strings.Split(remoteURL, "/")
		return parts[0]
	}
	return remoteURL
}

func init() {
	_ = bytes.Buffer{}
}
