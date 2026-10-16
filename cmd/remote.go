package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
	"github.com/noyzilla/oops/internal/sync"
	"github.com/spf13/cobra"
)

var (
	remoteConnectPathFlag string
)

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manages remote server environments and overlay capsules",
		Long:  "Commands for connecting remote servers, syncing/deploying workspace versions, and executing remote commands over SSH.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRemoteConnectCmd())
	cmd.AddCommand(newRemoteSetupCmd())
	cmd.AddCommand(newRemoteListCmd())
	cmd.AddCommand(newRemoteRemoveCmd())
	cmd.AddCommand(newRemoteRenameCmd())
	cmd.AddCommand(newRemotePushCmd())
	cmd.AddCommand(newRemotePullCmd())

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

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func parseRemoteConnectTarget(raw string) (string, string, string) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "@")

	var remotePath string
	if idx := strings.Index(clean, ":"); idx != -1 {
		rawPath := strings.TrimSpace(clean[idx+1:])
		clean = clean[:idx]
		if rawPath != "" {
			if !strings.HasPrefix(rawPath, "/") && !strings.HasPrefix(rawPath, "~") {
				remotePath = "~/" + rawPath
			} else {
				remotePath = rawPath
			}
		}
	}

	sshTarget := clean

	var capsuleName string
	if idx := strings.Index(clean, "@"); idx != -1 {
		capsuleName = clean[idx+1:]
	} else {
		capsuleName = clean
	}

	if idx := strings.Index(capsuleName, "/"); idx != -1 {
		capsuleName = capsuleName[:idx]
	}

	return sshTarget, capsuleName, remotePath
}

func capsuleHasContent(capsuleDir string) bool {
	entries, err := os.ReadDir(capsuleDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if name == "remote.yaml" {
			continue
		}
		if name == ".env" || !strings.HasPrefix(name, ".") {
			return true
		}
	}
	return false
}

func newRemoteConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect @<remote>[:<path>] [remote-path] [--path <remote-path>]",
		Short: "Connects to a remote server, inspects workspace, and syncs capsule",
		Long:  "Connects to remote server via SSH, detects existing Oopsbox workspace and local capsule, prompts or auto-syncs, and registers /@<host>/remote.yaml.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			rawRemote := args[0]
			sshTarget, capsuleName, inlinePath := parseRemoteConnectTarget(rawRemote)
			if capsuleName == "" {
				return fmt.Errorf("invalid remote target '%s'", rawRemote)
			}

			remotePath := inlinePath
			if remotePath == "" && len(args) > 1 && args[1] != "" {
				remotePath = args[1]
			}
			if remotePath == "" && cmd.Flags().Changed("path") {
				remotePath = remoteConnectPathFlag
			}
			if remotePath == "" {
				remotePath = remoteConnectPathFlag
			}
			if remotePath == "" {
				remotePath = "~/oopsbox"
			}

			capsuleDir := remote.CapsuleDir(workDirAbs, capsuleName)
			if _, err := os.Stat(capsuleDir); err == nil {
				cmd.Printf("Notice: Local capsule '@%s' already exists in this workspace.\n", capsuleName)
				cmd.Println("If connecting multiple environments to the same host, declare distinct Host entries in ~/.ssh/config.")
			}

			cmd.Printf("==> Connecting to remote server '@%s' (%s)...\n", capsuleName, sshTarget)

			// 1. Verify remote SSH connectivity & Docker availability
			pingCmd := "docker info >/dev/null 2>&1 || true"
			if out, err := remote.ExecuteRemoteSSH(sshTarget, pingCmd); err != nil {
				cmd.Printf("Notice: could not verify remote docker daemon on %s: %v (%s)\n", sshTarget, err, out)
			}

			// 2. Check and auto-bootstrap server prerequisites (docker compose plugin & oops CLI)
			ready, _ := remote.CheckServerPrerequisites(sshTarget)
			if !ready {
				cmd.Printf("==> Remote server '@%s' missing docker-compose-plugin or oops CLI. Running automated setup...\n", capsuleName)
				if err := remote.SetupServer(sshTarget, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
					cmd.Printf("Warning: automated setup encountered an error: %v\n", err)
				}
			}

			// 3. Check if remote server has Oopsbox workspace at remotePath
			checkCmd := fmt.Sprintf("test -d %s && (test -f %s/compose.yaml || test -f %s/.env || test -d %s/services) && echo 'EXISTS' || echo 'NOT_FOUND'",
				remotePath, remotePath, remotePath, remotePath)
			out, _ := remote.ExecuteRemoteSSH(sshTarget, checkCmd)
			remoteHasWorkspace := strings.Contains(out, "EXISTS")

			// 4. Check if local capsule already has content
			localHasFiles := capsuleHasContent(capsuleDir)

			// 5. Save/update remote.yaml first so sync operations can resolve config
			cfg := &remote.Config{
				Host: sshTarget,
				Path: remotePath,
			}
			if err := remote.SaveConfig(workDirAbs, capsuleName, cfg); err != nil {
				return err
			}

			// 5. First-time sync routing
			if remoteHasWorkspace && localHasFiles {
				cmd.Printf("Notice: Found existing Oopsbox on server (%s) and local capsule '/@%s/'.\n", remotePath, capsuleName)
				action := "pull"
				if isTerminal() {
					cmd.Print("Choose sync direction [push/pull] (default: pull): ")
					scanner := bufio.NewScanner(os.Stdin)
					if scanner.Scan() {
						input := strings.TrimSpace(strings.ToLower(scanner.Text()))
						if input == "push" {
							action = "push"
						}
					}
				}
				if action == "push" {
					cmd.Printf("==> Pushing configurations from local capsule '/@%s/' to server...\n", capsuleName)
					if err := sync.Push(sync.Options{WorkDir: workDirAbs, RemoteName: capsuleName, SyncAll: true}); err != nil {
						cmd.Printf("Warning: push error: %v\n", err)
					}
				} else {
					cmd.Printf("==> Pulling configurations from server into local capsule '/@%s/'...\n", capsuleName)
					if err := sync.Pull(sync.Options{WorkDir: workDirAbs, RemoteName: capsuleName, SyncAll: true}); err != nil {
						cmd.Printf("Warning: pull error: %v\n", err)
					}
				}
			} else if remoteHasWorkspace && !localHasFiles {
				cmd.Printf("==> Discovered existing Oopsbox on server (%s). Pulling configurations into local capsule '/@%s/'...\n", remotePath, capsuleName)
				if err := sync.Pull(sync.Options{WorkDir: workDirAbs, RemoteName: capsuleName, SyncAll: true}); err != nil {
					cmd.Printf("Warning: pull error: %v\n", err)
				}
			} else if !remoteHasWorkspace && localHasFiles {
				cmd.Printf("==> Remote workspace not found on server. Pushing local capsule configurations from '/@%s/' to server...\n", capsuleName)
				if err := sync.Push(sync.Options{WorkDir: workDirAbs, RemoteName: capsuleName, SyncAll: true}); err != nil {
					cmd.Printf("Warning: push error: %v\n", err)
				}
			} else {
				// Neither has files: initialize from blueprints
				created, err := remote.InitCapsuleBlueprints(workDirAbs, capsuleName)
				if err != nil {
					cmd.Printf("Notice: blueprint initialization: %v\n", err)
				} else if len(created) > 0 {
					cmd.Printf("==> Initialized capsule blueprints for /@%s/: %s\n", capsuleName, strings.Join(created, ", "))
				}
			}

			// Ensure blueprints exist even if pull happened
			if _, err := remote.InitCapsuleBlueprints(workDirAbs, capsuleName); err != nil {
				cmd.Printf("Notice: blueprint check: %v\n", err)
			}

			cmd.Printf("\n✓ Successfully connected remote '@%s'\n", capsuleName)
			cmd.Printf("  Host             : %s\n", cfg.Host)
			cmd.Printf("  Remote Path      : %s\n", cfg.Path)
			cmd.Printf("  Manifest         : /@%s/remote.yaml\n", capsuleName)
			cmd.Println()
			cmd.Printf("To deploy your workspace to '@%s', run:\n", capsuleName)
			cmd.Printf("  oops deploy @%s\n", capsuleName)
			return nil
		},
	}

	cmd.Flags().StringVar(&remoteConnectPathFlag, "path", "~/oopsbox", "Remote working directory path")
	return cmd
}

func newRemoteSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup [@<remote>]",
		Short: "Installs or upgrades docker-compose-plugin and oops CLI to latest version on remote server",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			var remoteName string
			if len(args) > 0 {
				remoteName = strings.TrimPrefix(args[0], "@")
			} else {
				remotes, err := remote.ListRemotes(workDirAbs)
				if err != nil {
					return err
				}
				if len(remotes) == 0 {
					return fmt.Errorf("no remote servers registered. Run 'oops remote connect @<remote>[:<path>]' first")
				}
				remoteName = remotes[0]
				for _, r := range remotes {
					if r == "prod" {
						remoteName = "prod"
						break
					}
				}
			}

			cfg, err := remote.LoadConfig(workDirAbs, remoteName)
			if err != nil {
				return fmt.Errorf("remote '@%s' not found: %w", remoteName, err)
			}

			cmd.Printf("==> Running environment setup on remote '@%s' (%s)...\n", remoteName, cfg.Host)
			if err := remote.SetupServer(cfg.Host, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return fmt.Errorf("remote setup failed on '@%s': %w", remoteName, err)
			}

			cmd.Printf("✓ Environment setup completed successfully on '@%s'\n", remoteName)
			return nil
		},
	}
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
				cmd.Println("No remote servers registered for this workspace.")
				cmd.Println("Connect a remote server using: oops remote connect @<remote> [path]")
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

func newRemoteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove @<remote>",
		Short: "Removes a registered remote server capsule from current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimPrefix(args[0], "@")
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			if err := remote.RemoveRemote(workDirAbs, name); err != nil {
				return fmt.Errorf("failed removing remote '@%s': %w", name, err)
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

			if _, err := os.Stat(oldCapsule); os.IsNotExist(err) {
				return fmt.Errorf("remote '@%s' not found", oldName)
			}
			if _, err := os.Stat(newCapsule); err == nil {
				return fmt.Errorf("remote '@%s' already exists", newName)
			}
			if err := os.Rename(oldCapsule, newCapsule); err != nil {
				return fmt.Errorf("failed renaming remote '@%s' to '@%s': %w", oldName, newName, err)
			}
			cmd.Printf("✓ Renamed remote '@%s' to '@%s'\n", oldName, newName)
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
	return "prod"
}

func resolveSSHTarget(workDir, serverName string) string {
	cleanName := strings.TrimPrefix(serverName, "@")
	cfg, err := remote.LoadConfig(workDir, cleanName)
	if err == nil && cfg.Host != "" {
		return cfg.Host
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

func parsePushPullArgs(args []string) (stack string, remoteName string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "/") || strings.Contains(arg, ",") {
			stack = strings.TrimPrefix(arg, "/")
		} else if strings.HasPrefix(arg, "@") {
			remoteName = strings.TrimPrefix(arg, "@")
		} else if remoteName == "" {
			remoteName = arg
		}
	}
	return stack, remoteName
}

func newRemotePushCmd() *cobra.Command {
	var remoteFlag string
	cmd := &cobra.Command{
		Use:   "push [/<stack1>,/<stack2>] [@<remote>]",
		Short: "Streams manifests and capsule overlay to remote server (pure delivery)",
		Long:  "Streams selective stack/service manifests and capsule overlay (.oopsallow) to remote server via SSH without restarting containers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			stack, rName := parsePushPullArgs(args)
			if remoteFlag != "" {
				rName = strings.TrimPrefix(remoteFlag, "@")
			}
			if rName == "" {
				rName = resolveDefaultServer(workDirAbs)
			}
			if rName == "" {
				return fmt.Errorf("no remote server specified. Run 'oops remote connect @<host>' first")
			}
			return executeRemotePush(workDirAbs, rName, stack)
		},
	}
	cmd.Flags().StringVarP(&remoteFlag, "remote", "r", "", "Target remote server name (default: @prod or first registered remote)")
	return cmd
}

func newRemotePullCmd() *cobra.Command {
	var remoteFlag string
	cmd := &cobra.Command{
		Use:   "pull [/<stack1>,/<stack2>] [@<remote>]",
		Short: "Downloads remote configurations into local capsule (filtered by .oopsallow)",
		Long:  "Downloads manifests, overrides, and configurations from the remote server into /@<remote>/ via SSH, strictly filtered by .oopsallow.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}
			stack, rName := parsePushPullArgs(args)
			if remoteFlag != "" {
				rName = strings.TrimPrefix(remoteFlag, "@")
			}
			if rName == "" {
				rName = resolveDefaultServer(workDirAbs)
			}
			if rName == "" {
				return fmt.Errorf("no remote server specified. Run 'oops remote connect @<host>' first")
			}
			return sync.Pull(sync.Options{
				WorkDir:    workDirAbs,
				RemoteName: rName,
				Stack:      stack,
			})
		},
	}
	cmd.Flags().StringVarP(&remoteFlag, "remote", "r", "", "Target remote server name (default: @prod or first registered remote)")
	return cmd
}

func executeRemotePush(workDirAbs, remoteName, stack string) error {
	remoteName = strings.TrimPrefix(remoteName, "@")

	cfg, err := remote.LoadConfig(workDirAbs, remoteName)
	if err != nil {
		return fmt.Errorf("remote '@%s' not registered: %w", remoteName, err)
	}

	fmt.Printf("==> Delivering workspace to remote '@%s' (%s, target path: %s)...\n", remoteName, cfg.Host, cfg.Path)

	// Step 1: Base Manifest Streaming (Selective copy)
	fmt.Println("==> [1/2] Streaming base manifests and required services...")
	inspector, err := remote.ResolveRequiredDependencies(workDirAbs, stack, remoteName)
	if err != nil {
		return fmt.Errorf("failed resolving service dependencies: %w", err)
	}

	var baseFiles []string
	baseFiles = append(baseFiles, inspector.StackFiles...)
	baseFiles = append(baseFiles, inspector.ServiceFiles...)
	baseFiles = append(baseFiles, inspector.ConfigFiles...)

	if len(baseFiles) > 0 {
		fmt.Printf("    Selective Base: %d stack manifest(s), %d service template(s), %d config dir(s)\n",
			len(inspector.StackFiles), len(inspector.ServiceFiles), len(inspector.ConfigFiles))
		if err := remote.StreamTarFilesSSH(workDirAbs, baseFiles, cfg.Host, cfg.Path); err != nil {
			return fmt.Errorf("base manifest streaming failed: %w", err)
		}
	}

	// Step 2: Capsule Overlay Streaming (Strictly filtered by .oopsallow)
	fmt.Printf("==> [2/2] Applying capsule overlay from /@%s/ (via .oopsallow)...\n", remoteName)
	if err := sync.Push(sync.Options{
		WorkDir:        workDirAbs,
		RemoteName:     remoteName,
		Stack:          stack,
		RequiredStacks: inspector.RequiredStacks,
	}); err != nil {
		return fmt.Errorf("capsule overlay push failed: %w", err)
	}

	// Post-Delivery Hint (Pure delivery - no automatic up -d)
	fmt.Printf("\n✓ Successfully delivered workspace to remote '@%s' (%s)\n", remoteName, cfg.Host)
	if stack != "" {
		targets := remote.ParseStackTargets(stack)
		var prefixed []string
		for _, t := range targets {
			prefixed = append(prefixed, "/"+t)
		}
		targetStr := strings.Join(prefixed, ",")
		fmt.Printf("  Run 'oops rx @%s up %s' to start or restart services.\n", remoteName, targetStr)
	} else {
		fmt.Printf("  Run 'oops rx @%s up' to start or restart services.\n", remoteName)
	}
	return nil
}
