package cmd

import (
	"fmt"

	"github.com/noyzilla/oops/internal/sync"
	"github.com/spf13/cobra"
)

var syncRemoteFlag string

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronizes configurations (.env) and sensitive data (config/secrets/) with remote server",
		Long:  "Synchronizes configurations and sensitive data using the Suffix Mapping Protocol (.<remote>).",
	}

	cmd.PersistentFlags().StringVarP(&syncRemoteFlag, "remote", "r", "", "Target remote server (e.g., prod)")

	// Root sync push/pull (syncs BOTH)
	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push both env and secret files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(true, true, true)
		},
	}

	pullCmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull both env and secret files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(false, true, true)
		},
	}

	// Env subcommands
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Sync only environment configurations (.env)",
	}
	envCmd.AddCommand(&cobra.Command{
		Use:   "push",
		Short: "Push env files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(true, true, false)
		},
	})
	envCmd.AddCommand(&cobra.Command{
		Use:   "pull",
		Short: "Pull env files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(false, true, false)
		},
	})

	// Secret subcommands
	secretCmd := &cobra.Command{
		Use:   "secret",
		Short: "Sync only secret files (config/secrets/)",
	}
	secretCmd.AddCommand(&cobra.Command{
		Use:   "push",
		Short: "Push secret files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(true, false, true)
		},
	})
	secretCmd.AddCommand(&cobra.Command{
		Use:   "pull",
		Short: "Pull secret files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(false, false, true)
		},
	})

	cmd.AddCommand(pushCmd, pullCmd, envCmd, secretCmd)
	return cmd
}

func runSync(isPush, doEnv, doSecret bool) error {
	workDirAbs, err := RequireGitOopsboxWorkspace(targetDir)
	if err != nil {
		return err
	}

	remoteName := syncRemoteFlag
	if remoteName == "" {
		remoteName = resolveDefaultServer(workDirAbs)
	}

	if remoteName == "" {
		return fmt.Errorf("no remote specified and no default remote found")
	}

	sshTarget := resolveSSHTarget(workDirAbs, remoteName)
	if sshTarget == "" || sshTarget == remoteName {
		// If fallback triggers and just returns the name, it might be correct or it might just be unresolved.
		// For safety, we can just use the returned sshTarget but print what we resolved.
		// sshTarget = extractSSHTarget(serverName) which returns remoteName if it cannot find git remote
	}

	opts := sync.Options{
		WorkDir:    workDirAbs,
		RemoteName: remoteName,
		SSHTarget:  sshTarget,
		SyncEnv:    doEnv,
		SyncSecret: doSecret,
	}

	if isPush {
		fmt.Printf("Pushing to remote %s (target: %s)...\n", remoteName, sshTarget)
		return sync.Push(opts)
	}

	fmt.Printf("Pulling from remote %s (target: %s)...\n", remoteName, sshTarget)
	return sync.Pull(opts)
}
