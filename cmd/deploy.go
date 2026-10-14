package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deployRemoteFlag string
)

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [/<stack>] [@<remote>]",
		Short: "Deploys workspace updates to a remote server over SSH (alias for 'oops remote push')",
		Long:  "Streams selective stack/service manifests and capsule overlays (.oopsallow) via SSH (pure delivery, no auto-start). Exact ergonomic alias for 'oops remote push'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirAbs, err := RequireOopsboxWorkspace(targetDir)
			if err != nil {
				return err
			}

			stack, remoteTarget := parsePushPullArgs(args)

			if deployRemoteFlag != "" {
				remoteTarget = strings.TrimPrefix(deployRemoteFlag, "@")
			}

			if remoteTarget == "" {
				remoteTarget = resolveDefaultServer(workDirAbs)
			}

			if remoteTarget == "" {
				return fmt.Errorf("no remote server specified and no registered remote found. Run 'oops remote connect @<host>' first")
			}

			return executeRemotePush(workDirAbs, remoteTarget, stack)
		},
	}

	cmd.Flags().StringVarP(&deployRemoteFlag, "remote", "r", "", "Target remote server name (default: @prod or first registered remote)")
	return cmd
}
