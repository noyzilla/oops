package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
	"github.com/spf13/cobra"
)

var (
	deployRemoteFlag string
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [/<project>] [@<remote>]",
		Short: "Deploys workspace updates to a remote server over SSH",
		Long:  "Streams base manifests and remote overlays directly via SSH and executes remote docker compose up.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			var projectTarget string
			var remoteTarget string

			for _, arg := range args {
				if strings.HasPrefix(arg, "/") {
					projectTarget = strings.TrimPrefix(arg, "/")
				} else if strings.HasPrefix(arg, "@") {
					remoteTarget = strings.TrimPrefix(arg, "@")
				} else {
					if remoteTarget == "" {
						remoteTarget = arg
					}
				}
			}

			if deployRemoteFlag != "" {
				remoteTarget = strings.TrimPrefix(deployRemoteFlag, "@")
			}

			if remoteTarget == "" {
				remoteTarget = resolveDefaultServer(workDirAbs)
			}

			if remoteTarget == "" {
				return fmt.Errorf("no remote server specified and no registered remote found. Run 'oops remote add <ssh-target> [@<name>]' first")
			}

			return executeDeploy(workDirAbs, remoteTarget, projectTarget)
		},
	}

	cmd.Flags().StringVarP(&deployRemoteFlag, "remote", "r", "", "Target remote server name (default: @prod or first registered remote)")
	return cmd
}

func executeDeploy(workDirAbs, remoteName, projectTarget string) error {
	remoteName = strings.TrimPrefix(remoteName, "@")

	// 1. Load remote config from /@<name>/remote.yaml
	cfg, err := remote.LoadConfig(workDirAbs, remoteName)
	if err != nil {
		// Fallback check if it's a legacy git remote
		sshTarget := resolveSSHTarget(workDirAbs, remoteName)
		if sshTarget == "" || sshTarget == remoteName {
			return fmt.Errorf("remote '@%s' not registered. Run 'oops remote add <ssh-target> @%s' first", remoteName, remoteName)
		}
		cfg = &remote.Config{
			Host: sshTarget,
			Path: "~/oopsbox",
		}
	}

	fmt.Printf("==> Deploying workspace to remote '@%s' (%s, target path: %s)...\n", remoteName, cfg.Host, cfg.Path)

	// Step 1: Base Manifest Streaming
	// Stream base files (compose.yaml, services/, config/, etc.) excluding *.local.yaml, @*, .git, .scratch, data
	fmt.Println("==> [1/3] Streaming base manifests...")
	excludes := []string{
		"*.local.yaml",
		"@*",
		".git",
		".scratch",
		"data",
	}
	if err := remote.StreamTarSSH(workDirAbs, excludes, cfg.Host, cfg.Path); err != nil {
		return fmt.Errorf("base manifest streaming failed: %w", err)
	}

	// Step 2: Overlay Application
	// Stream overlay from /@<remote>/ excluding remote.yaml
	capsuleDir := remote.CapsuleDir(workDirAbs, remoteName)
	if fi, err := os.Stat(capsuleDir); err == nil && fi.IsDir() {
		fmt.Printf("==> [2/3] Applying remote overlay manifests from /@%s/...\n", remoteName)
		overlayExcludes := []string{
			"remote.yaml",
		}
		if err := remote.StreamTarSSH(capsuleDir, overlayExcludes, cfg.Host, cfg.Path); err != nil {
			return fmt.Errorf("overlay manifest streaming failed: %w", err)
		}
	} else {
		fmt.Println("==> [2/3] No overlay directory found, skipping overlay...")
	}

	// Step 3: Container Execution
	fmt.Println("==> [3/3] Orchestrating remote containers (docker compose up -d)...")
	var composeCmd string
	if projectTarget != "" {
		composeCmd = fmt.Sprintf(
			"cd %s && (if [ -f projects/%s/compose.yaml ]; then docker compose -f compose.yaml -f projects/%s/compose.yaml up -d; else docker compose up -d; fi)",
			cfg.Path, projectTarget, projectTarget,
		)
	} else {
		composeCmd = fmt.Sprintf("cd %s && docker compose up -d", cfg.Path)
	}

	if err := remote.ExecuteSSH(cfg.Host, composeCmd, os.Stdout, os.Stderr); err != nil {
		return fmt.Errorf("remote docker compose up failed: %w", err)
	}

	fmt.Printf("\n✓ Successfully deployed workspace to remote '@%s' (%s)\n", remoteName, cfg.Host)
	return nil
}
