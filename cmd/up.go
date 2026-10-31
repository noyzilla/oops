package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var delay string
	var detach bool

	cmd := &cobra.Command{
		Use:   "up [targets...]",
		Short: "Starts stack or targeted services",
		Long:  "Starts stack or specific services. Target can be a whole stack (e.g. /apps, /db), scoped service (e.g. /db/mysql), service name (e.g. mysql), or wildcard (e.g. app..). By default, tails service logs after starting. Use --detach to run in background.",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}
			targets, err := docker.ResolveTargets(workDir, args)
			if err != nil {
				return err
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()
			orch.WorkDir = workDir

			if err := orch.Up(context.Background(), targets, d); err != nil {
				return err
			}

			if detach {
				return nil
			}

			return runLogs(cmd, args, 50, true)
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive starts (e.g. 5s, 5)")
	cmd.Flags().BoolVar(&detach, "detach", false, "Run containers in the background without tailing logs")

	return cmd
}
