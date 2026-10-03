package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var delay string

	cmd := &cobra.Command{
		Use:   "update <targets...>",
		Short: "Executes sequential rolling update with health check polling",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			targets, err := docker.ResolveTargets(".", args)
			if err != nil {
				return err
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()

			return orch.Update(context.Background(), targets, d)
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive updates (e.g. 5s, 5)")

	return cmd
}
