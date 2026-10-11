package cmd

import (
	"fmt"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
	"github.com/noyzilla/oops/internal/sync"
	"github.com/spf13/cobra"
)

func parseSyncRemote(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			return strings.TrimPrefix(arg, "@")
		}
		if arg != "" && !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronizes configurations (.env) and sensitive data (config/secrets/) with remote server",
		Long:  "Synchronizes configurations and sensitive data using the 1:1 Mirror Directory Protocol (/@<remote>/).",
	}

	// Root sync push/pull (syncs BOTH)
	pushCmd := &cobra.Command{
		Use:   "push [@<remote>]",
		Short: "Push both env and secret files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), true, true, true)
		},
	}

	pullCmd := &cobra.Command{
		Use:   "pull [@<remote>]",
		Short: "Pull both env and secret files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), false, true, true)
		},
	}

	// Env subcommands
	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Sync only environment configurations (.env)",
	}
	envCmd.AddCommand(&cobra.Command{
		Use:   "push [@<remote>]",
		Short: "Push env files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), true, true, false)
		},
	})
	envCmd.AddCommand(&cobra.Command{
		Use:   "pull [@<remote>]",
		Short: "Pull env files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), false, true, false)
		},
	})

	// Secret subcommands
	secretCmd := &cobra.Command{
		Use:   "secret",
		Short: "Sync only secret files (config/secrets/)",
	}
	secretCmd.AddCommand(&cobra.Command{
		Use:   "push [@<remote>]",
		Short: "Push secret files to remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), true, false, true)
		},
	})
	secretCmd.AddCommand(&cobra.Command{
		Use:   "pull [@<remote>]",
		Short: "Pull secret files from remote",
		RunE: func(c *cobra.Command, args []string) error {
			return runSync(parseSyncRemote(args), false, false, true)
		},
	})

	cmd.AddCommand(pushCmd, pullCmd, envCmd, secretCmd)
	return cmd
}

func runSync(remoteTarget string, isPush, doEnv, doSecret bool) error {
	workDirAbs, err := RequireOopsboxWorkspace(targetDir)
	if err != nil {
		return err
	}

	remoteName := strings.TrimPrefix(remoteTarget, "@")
	if remoteName == "" {
		remotes, _ := remote.ListRemotes(workDirAbs)
		for _, r := range remotes {
			if r == "prod" {
				remoteName = "prod"
				break
			}
		}
		if remoteName == "" && len(remotes) > 0 {
			remoteName = remotes[0]
		}
		if remoteName == "" {
			remoteName = resolveDefaultServer(workDirAbs)
		}
	}

	if remoteName == "" {
		return fmt.Errorf("no remote specified and no registered remote found")
	}

	opts := sync.Options{
		WorkDir:    workDirAbs,
		RemoteName: remoteName,
		SyncEnv:    doEnv,
		SyncSecret: doSecret,
		SyncAll:    doEnv && doSecret,
	}

	if isPush {
		return sync.Push(opts)
	}

	return sync.Pull(opts)
}
