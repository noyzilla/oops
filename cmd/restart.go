package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	var delay string

	cmd := &cobra.Command{
		Use:   "restart [targets...]",
		Short: "Restarts target services with pre-stop hooks and health checks",
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

			return orch.Restart(context.Background(), targets, d)
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive restarts (e.g. 5s, 5)")

	return cmd
}
