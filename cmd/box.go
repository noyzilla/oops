package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

var boxCloneRemoteFlag string

func newBoxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "box",
		Short: "Manages Oopsbox developer workspaces and local environments",
		Long:  "Commands for creating, starting, switching, listing, and managing local Oopsbox environments.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newBoxInitCmd())
	cmd.AddCommand(newBoxCloneCmd())
	cmd.AddCommand(newBoxActiveCmd())
	cmd.AddCommand(newBoxListCmd())
	cmd.AddCommand(newBoxStartCmd())
	cmd.AddCommand(newBoxStopCmd())
	cmd.AddCommand(newBoxSwitchCmd())
	cmd.AddCommand(newBoxCertCmd())

	return cmd
}

func newBoxInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initializes a new Oopsbox workspace from the latest release blueprint",
		Long:  "Downloads the latest official oopsbox release template, generates secure credentials (.env), initializes config, and sets active box.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}
			createdDir, err := box.CreateWorkspace(targetPath)
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Printf("==> Oopsbox workspace successfully created at: %s\n", createdDir)
			fmt.Println("To boot your new environment, run:")
			fmt.Printf("  cd %s && oops box start\n", createdDir)
			fmt.Println("Or start it directly:")
			fmt.Printf("  oops box start -C %s\n", createdDir)
			return nil
		},
	}
}

func newBoxActiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "active [path]",
		Short: "Displays or sets the currently active Oopsbox workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				active, err := box.GetActiveBox()
				if err != nil {
					return err
				}
				if active == "" {
					fmt.Println("No active Oopsbox workspace set in ~/.oops/active_box")
					return nil
				}
				valid := box.HasValidStacks(active)
				fmt.Printf("Active Box : %s\n", active)
				if !valid {
					fmt.Printf("Status     : Warning: Directory does not contain valid compose stacks (stacks/)\n")
				} else {
					fmt.Printf("Status     : Ready (Valid stacks discovered)\n")
				}
				return nil
			}

			targetPath := args[0]
			canon := box.CanonicalPath(targetPath)
			if !box.HasValidStacks(canon) {
				return fmt.Errorf("target directory %s is not a valid Oopsbox workspace (missing stacks/)", canon)
			}

			if err := box.SetActiveBox(canon); err != nil {
				return fmt.Errorf("failed to set active box: %w", err)
			}
			fmt.Printf("Active Oopsbox workspace set to: %s\n", canon)
			return nil
		},
	}
}

func newBoxListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists discovered and active Oopsbox workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			active, _ := box.GetActiveBox()
			workspaces := box.DiscoverWorkspaces()

			if len(workspaces) == 0 {
				fmt.Println("No Oopsbox workspaces discovered.")
				fmt.Println("Create a new workspace using: oops box init [path]")
				return nil
			}

			fmt.Println("Discovered Oopsbox Workspaces:")
			for _, ws := range workspaces {
				if active != "" && box.CanonicalPath(ws) == box.CanonicalPath(active) {
					fmt.Printf("  * %s (active)\n", ws)
				} else {
					fmt.Printf("    %s\n", ws)
				}
			}
			return nil
		},
	}
}

func newBoxStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Boots container engine, configures DNS resolver, and starts default stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			if !box.HasValidStacks(workDir) {
				return fmt.Errorf("no valid Oopsbox workspace found at %s", workDir)
			}

			cfg, err := box.LoadOopsboxConfig(workDir)
			if err != nil {
				return err
			}

			// 1. Ensure engine is running
			engine, err := box.EnsureEngineStarted(cfg)
			if err != nil {
				return fmt.Errorf("engine startup failed (%s): %w", engine, err)
			}
			log.Printf("==> Container Engine: %s", engine)

			// 2. Set active box
			_ = box.SetActiveBox(workDir)

			// 3. Start default stack
			targets, err := docker.ResolveTargets(workDir, nil)
			if err != nil {
				return fmt.Errorf("failed resolving startup targets: %w", err)
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()
			orch.WorkDir = workDir

			log.Printf("==> Starting Oopsbox services (%d targets)...", len(targets))
			upErr := orch.Up(context.Background(), targets, 0)
			var blockedErr *orchestrator.BlockedError
			if upErr != nil && !errors.As(upErr, &blockedErr) {
				return upErr
			}

			// Load OopsConfig to get DNS.TLD
			oopsCfg, err := docker.LoadOopsConfig(workDir)
			if err != nil {
				oopsCfg = &docker.OopsConfig{}
				oopsCfg.DNS.TLD = "oops"
			}

			// 4. Setup macOS resolver with active container IP
			if err := box.SetupMacOSResolver(oopsCfg.DNS.TLD); err != nil {
				log.Printf("Warning: DNS resolver setup notice: %v", err)
			}

			// 5. Sync static DNS records
			if err := box.SyncStaticDNSRecords(workDir, oopsCfg.DNS.TLD); err != nil {
				log.Printf("Warning: DNS static record sync notice: %v", err)
			}

			// 6. Setup Colima routing if applicable
			_ = box.SetupColimaRouting()

			return upErr
		},
	}
}

func stopWorkspaceStacks(dir string) error {
	orderedStacks, composeMap, err := docker.DiscoverStacks(dir)
	if err != nil {
		return err
	}

	var reverseStacks []string
	for i := len(orderedStacks) - 1; i >= 0; i-- {
		reverseStacks = append(reverseStacks, orderedStacks[i])
	}

	for _, stack := range reverseStacks {
		composePath := composeMap[stack]
		log.Printf("==> Tearing down stack /%s...", stack)
		if err := orchestrator.RunComposeCommand(composePath, "down"); err != nil {
			log.Printf("Warning: down failed for /%s: %v", stack, err)
		}
	}
	return nil
}

func newBoxStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops all running stacks in the active workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			if !box.HasValidStacks(workDir) {
				return fmt.Errorf("no valid Oopsbox workspace found at %s", workDir)
			}

			log.Println("==> Stopping Oopsbox workspace services...")
			return stopWorkspaceStacks(workDir)
		},
	}
}

func newBoxSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <path>",
		Short: "Gracefully switches active workspace: stops current box and boots target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := args[0]
			canonTarget := box.CanonicalPath(targetPath)
			if !box.HasValidStacks(canonTarget) {
				return fmt.Errorf("target directory %s is not a valid Oopsbox workspace", canonTarget)
			}

			active, _ := box.GetActiveBox()
			if active != "" && box.CanonicalPath(active) == canonTarget {
				fmt.Printf("Workspace %s is already active.\n", canonTarget)
				return nil
			}

			// 1. Gracefully teardown current active workspace if running
			if active != "" && box.HasValidStacks(active) {
				log.Printf("==> [Switch] Tearing down current active workspace: %s", active)
				_ = stopWorkspaceStacks(active)
			}

			// 2. Set new active box
			if err := box.SetActiveBox(canonTarget); err != nil {
				return fmt.Errorf("failed setting active box: %w", err)
			}

			// 3. Boot new workspace
			log.Printf("==> [Switch] Booting target workspace: %s", canonTarget)
			cfg, err := box.LoadOopsboxConfig(canonTarget)
			if err != nil {
				return err
			}

			if _, err := box.EnsureEngineStarted(cfg); err != nil {
				return err
			}

			targets, err := docker.ResolveTargets(canonTarget, nil)
			if err != nil {
				return err
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()
			orch.WorkDir = canonTarget

			upErr := orch.Up(context.Background(), targets, 0)
			var blockedErr *orchestrator.BlockedError
			if upErr != nil && !errors.As(upErr, &blockedErr) {
				return upErr
			}

			oopsCfg, _ := docker.LoadOopsConfig(canonTarget)
			tld := "oops"
			if oopsCfg != nil && oopsCfg.DNS.TLD != "" {
				tld = oopsCfg.DNS.TLD
			}
			_ = box.SetupMacOSResolver(tld)
			_ = box.SyncStaticDNSRecords(canonTarget, tld)
			_ = box.SetupColimaRouting()

			log.Printf("==> [Switch] Successfully switched active workspace to: %s", canonTarget)
			return upErr
		},
	}
}

func newBoxCertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cert",
		Short: "Installs local Caddy CA root certificate into host OS trust store / Keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			if !box.HasValidStacks(workDir) {
				return fmt.Errorf("no valid Oopsbox workspace found at %s", workDir)
			}
			return box.InstallCACertificate(workDir)
		},
	}
}

func newBoxCloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <ssh-target> [-r <remote-name>] [path]",
		Short: "Clones an Oopsbox workspace from a remote server over SSH",
		Long:  "Clones workspace from a remote server via SSH, registers remote target in local Git config, and prepares environment.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sshTarget := args[0]
			targetPath := ""
			if len(args) > 1 {
				targetPath = args[1]
			}
			return executeBoxClone(sshTarget, boxCloneRemoteFlag, targetPath)
		},
	}
	cmd.Flags().StringVarP(&boxCloneRemoteFlag, "remote", "r", "", "Target remote server logical name (default: prod)")
	return cmd
}

func deriveCloneTargetPath(sshTarget, targetPathArg string) string {
	if targetPathArg != "" {
		return targetPathArg
	}

	cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
	if idx := strings.Index(cleanTarget, "/"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}
	if idx := strings.Index(cleanTarget, ":"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}
	if idx := strings.Index(cleanTarget, "@"); idx != -1 {
		cleanTarget = cleanTarget[idx+1:]
	}

	if cleanTarget == "" {
		return "oopsbox"
	}
	return cleanTarget
}

func executeBoxClone(sshTarget, remoteFlag, targetPathArg string) error {
	targetPath := deriveCloneTargetPath(sshTarget, targetPathArg)
	targetPathAbs, err := filepath.Abs(targetPath)
	if err != nil {
		targetPathAbs = targetPath
	}

	if fi, err := os.Stat(targetPathAbs); err == nil {
		if fi.IsDir() {
			entries, err := os.ReadDir(targetPathAbs)
			if err == nil && len(entries) > 0 {
				return fmt.Errorf("destination path '%s' already exists and is not an empty directory", targetPath)
			}
		} else {
			return fmt.Errorf("destination path '%s' already exists and is not a directory", targetPath)
		}
	}

	remoteName := remoteFlag
	if remoteName == "" {
		remoteName = "prod"
	}

	gitRemoteName := toGitRemoteName(remoteName)
	cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
	if idx := strings.Index(cleanTarget, "/"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}

	fmt.Printf("==> Cloning Oopsbox workspace from '%s' into '%s'...\n", sshTarget, targetPath)

	bareURL := fmt.Sprintf("%s:.oops/oopsbox.git", cleanTarget)
	cloneCmd := exec.Command("git", "clone", bareURL, targetPathAbs)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr

	if err := cloneCmd.Run(); err != nil {
		fmt.Println("Notice: Bare repository not found at ~/.oops/oopsbox.git, attempting fallback to ~/oopsbox...")
		workingURL := fmt.Sprintf("%s:~/oopsbox", cleanTarget)
		fallbackCmd := exec.Command("git", "clone", workingURL, targetPathAbs)
		fallbackCmd.Stdout = os.Stdout
		fallbackCmd.Stderr = os.Stderr

		if err := fallbackCmd.Run(); err != nil {
			return fmt.Errorf("failed cloning workspace from '%s' (%s or %s): %w", sshTarget, bareURL, workingURL, err)
		}
	}

	_ = exec.Command("git", "-C", targetPathAbs, "remote", "remove", gitRemoteName).Run()
	_ = exec.Command("git", "-C", targetPathAbs, "remote", "remove", remoteName).Run()

	addRemoteCmd := exec.Command("git", "-C", targetPathAbs, "remote", "add", gitRemoteName, bareURL)
	if out, err := addRemoteCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cloned successfully but failed setting remote '%s': %s (%w)", gitRemoteName, string(out), err)
	}

	fmt.Println()
	fmt.Printf("✓ Successfully cloned Oopsbox workspace from '%s' to '%s'\n", sshTarget, targetPath)
	fmt.Printf("✓ Registered remote target '%s' (%s -> %s)\n", remoteName, gitRemoteName, bareURL)
	fmt.Println()
	fmt.Println("To edit and deploy updates:")
	fmt.Printf("  cd %s\n", targetPath)
	fmt.Printf("  oops deploy -r %s\n", remoteName)

	return nil
}
