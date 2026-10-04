package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var delay string
	var exceptTargets []string

	cmd := &cobra.Command{
		Use:   "stop [targets...]",
		Short: "Gracefully stops target services with pre-stop hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			targets, err := docker.ResolveTargetsWithExceptions(workDir, args, exceptTargets)
			if err != nil {
				return err
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()

			return orch.Stop(context.Background(), targets, d)
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive stops (e.g. 5s, 5)")
	cmd.Flags().StringSliceVarP(&exceptTargets, "except", "x", nil, "Exclude specific targets/groups/stacks from being stopped (e.g. -x @core, --except /db)")
	cmd.Flags().StringSliceVar(&exceptTargets, "exclude", nil, "Alias for --except")

	return cmd
}
