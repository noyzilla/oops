package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deployRemoteFlag string
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [-r <remote>] [ref]",
		Short: "Deploys workspace updates to a remote server over SSH",
		Long:  "Pushes Git history to remote bare repo and executes remote checkout and container orchestration (oops up). Use -r <remote> to specify target remote.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			remoteName := deployRemoteFlag
			if remoteName == "" {
				remoteName = resolveDefaultServer(workDirAbs)
			}

			ref := getCurrentGitBranch(workDirAbs)
			if ref == "" {
				ref = "main"
			}

			if len(args) == 1 {
				ref = args[0]
			}

			return executeDeploy(workDirAbs, remoteName, ref)
		},
	}

	cmd.Flags().StringVarP(&deployRemoteFlag, "remote", "r", "", "Target remote server name (default: oopsbox or first registered remote)")
	return cmd
}

func executeDeploy(workDirAbs, remoteName, ref string) error {
	gitRemoteName := toGitRemoteName(remoteName)
	remoteURLCmd := exec.Command("git", "-C", workDirAbs, "remote", "get-url", gitRemoteName)
	out, err := remoteURLCmd.Output()
	if err != nil {
		// Fallback to un-prefixed name
		gitRemoteName = remoteName
		remoteURLCmd = exec.Command("git", "-C", workDirAbs, "remote", "get-url", gitRemoteName)
		out, err = remoteURLCmd.Output()
	}
	if err != nil {
		return fmt.Errorf("remote '%s' not registered. Run 'oops remote add <ssh-target> -r %s' first", remoteName, remoteName)
	}

	rawURL := strings.TrimSpace(string(out))
	sshTarget := extractSSHTarget(rawURL)

	fmt.Printf("==> Deploying workspace to remote '%s' (%s, ref: %s)...\n", remoteName, sshTarget, ref)

	gitPushCmd := exec.Command("git", "-C", workDirAbs, "push", gitRemoteName, ref)
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr
	if err := gitPushCmd.Run(); err != nil {
		return fmt.Errorf("failed pushing git commits to remote '%s': %w", remoteName, err)
	}

	remoteDeployScript := fmt.Sprintf(
		"git --git-dir=$HOME/.oops/oopsbox.git --work-tree=$HOME/oopsbox checkout -f %s && "+
			"(command -v oops >/dev/null 2>&1 && oops up -C $HOME/oopsbox || /var/lib/google/bin/oops up -C $HOME/oopsbox)",
		ref,
	)

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		sshTarget,
		remoteDeployScript,
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("remote deployment execution failed: %w", err)
	}

	fmt.Printf("\n✓ Successfully deployed workspace to remote '%s' (%s)\n", remoteName, sshTarget)
	return nil
}

func isRegisteredRemote(workDir, remoteName string) bool {
	err := exec.Command("git", "-C",workDir, "remote", "get-url", remoteName).Run()
	return err == nil
}

func getCurrentGitBranch(workDir string) string {
	out, err := exec.Command("git", "-C", workDir, "branch", "--show-current").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}
