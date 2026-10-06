package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/key"
	"github.com/spf13/cobra"
)

func newBoxCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <repo> [path]",
		Short: "Clones an Oopsbox workspace repository using SSH Deploy Key",
		Long:  "Clones a Git repository using the local Oops SSH Deploy Key (~/.oops/id_ed25519). If authentication fails, displays the Deploy Key and direct link to add it.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			targetPath := "."
			if len(args) > 1 {
				targetPath = args[1]
			}

			keyPath, err := key.DefaultKeyPath()
			if err != nil {
				return err
			}

			pubKeyStr, _, err := key.EnsureKeyPair(keyPath)
			if err != nil {
				return err
			}

			targetAbs, err := filepath.Abs(targetPath)
			if err != nil {
				targetAbs = targetPath
			}

			cmd.Printf("==> Cloning workspace %s -> %s...\n", repoURL, targetAbs)

			// Construct git clone command using explicit SSH key
			sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new", keyPath)
			gitCmd := exec.Command("git", "clone", repoURL, targetPath)
			gitCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr

			if err := gitCmd.Run(); err != nil {
				cmd.Println("\n❌ Git clone failed. Authentication or access permission error.")
				cmd.Println("\n----------------------------------------------------------------------")
				cmd.Println("  ACTION REQUIRED: Add SSH Deploy Key to Git Repository")
				cmd.Println("----------------------------------------------------------------------")
				cmd.Println("Public Deploy Key (~/.oops/id_ed25519.pub):")
				cmd.Println(pubKeyStr)
				cmd.Println()

				if addURL := key.GetAddDeployKeyURL(repoURL); addURL != "" {
					cmd.Println("Direct URL to add Deploy Key:")
					cmd.Println("👉 " + addURL)
					cmd.Println()
				}

				cmd.Println("Instructions:")
				cmd.Println(" 1. Copy the Public Deploy Key above.")
				cmd.Println(" 2. Open the URL above (or go to Repo Settings -> Deploy Keys).")
				cmd.Println(" 3. Add a new Deploy Key (Read-Only access is sufficient).")
				cmd.Println(" 4. Re-run: oops box clone", repoURL, targetPath)
				cmd.Println("----------------------------------------------------------------------")
				return fmt.Errorf("git clone failed for repository %s", repoURL)
			}

			cmd.Printf("\n✓ Successfully cloned %s to %s\n", repoURL, targetAbs)

			// If the cloned directory contains a valid workspace, set it active and ensure .env exists
			if box.HasValidStacks(targetAbs) {
				_ = box.SetActiveBox(targetAbs)
				cmd.Printf("==> Set active box to: %s\n", targetAbs)
			}

			return nil
		},
	}
}

func newBoxPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull [path]",
		Short: "Pulls latest Git updates for an Oopsbox workspace using SSH Deploy Key",
		Long:  "Executes git pull in the workspace directory using the local Oops SSH Deploy Key (~/.oops/id_ed25519).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			customDir := ""
			if len(args) > 0 {
				customDir = args[0]
			}
			workDir := ResolveWorkDir(customDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			keyPath, err := key.DefaultKeyPath()
			if err != nil {
				return err
			}

			pubKeyStr, _, err := key.EnsureKeyPair(keyPath)
			if err != nil {
				return err
			}

			workDirAbs, err := filepath.Abs(workDir)
			if err != nil {
				workDirAbs = workDir
			}

			cmd.Printf("==> Pulling Git updates for Oopsbox at: %s...\n", workDirAbs)

			// Get remote URL to display direct add key link if authentication fails
			remoteCmd := exec.Command("git", "-C", workDirAbs, "remote", "get-url", "origin")
			remoteOut, _ := remoteCmd.Output()
			repoURL := string(remoteOut)

			sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new", keyPath)
			gitCmd := exec.Command("git", "-C", workDirAbs, "pull")
			gitCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr

			if err := gitCmd.Run(); err != nil {
				cmd.Println("\n❌ Git pull failed. Authentication or permission error.")
				cmd.Println("\n----------------------------------------------------------------------")
				cmd.Println("  ACTION REQUIRED: Verify SSH Deploy Key in Git Repository")
				cmd.Println("----------------------------------------------------------------------")
				cmd.Println("Public Deploy Key (~/.oops/id_ed25519.pub):")
				cmd.Println(pubKeyStr)
				cmd.Println()

				if addURL := key.GetAddDeployKeyURL(repoURL); addURL != "" {
					cmd.Println("Direct URL to add Deploy Key:")
					cmd.Println("👉 " + addURL)
					cmd.Println()
				}

				cmd.Println("----------------------------------------------------------------------")
				return fmt.Errorf("git pull failed for %s", workDirAbs)
			}

			cmd.Printf("\n✓ Successfully updated %s\n", workDirAbs)
			return nil
		},
	}
}
