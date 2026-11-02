package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var wait string
	var noFollow bool

	cmd := &cobra.Command{
		Use:   "up [targets...]",
		Short: "Starts stack or targeted services",
		Long:  "Starts stack or specific services. Target can be a whole stack (e.g. /apps, /db), scoped service (e.g. /db/mysql), service name (e.g. mysql), or wildcard (e.g. app..). By default, tails service logs after starting. Use -d, --no-follow to run without following logs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(wait)
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

			if noFollow {
				return nil
			}

			return runLogs(cmd, args, 50, true)
		},
	}

	cmd.Flags().StringVarP(&wait, "wait", "w", "0s", "Inter-service wait duration between consecutive starts (e.g. 5s, 5)")
	cmd.Flags().BoolVarP(&noFollow, "no-follow", "d", false, "Do not follow service logs after starting")

	return cmd
}
