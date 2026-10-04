package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	var delay string
	var imageFlag string

	cmd := &cobra.Command{
		Use:   "restart [targets...]",
		Short: "Restarts target services with pre-stop hooks and health checks",
		Long:  "Restarts target services, stacks, groups, or containers matching a specific image with pre-stop hooks and health checks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if imageFlag != "" {
				args = append(args, "img:"+imageFlag)
			}

			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			targets, err := docker.ResolveTargets(workDir, args)
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
	cmd.Flags().StringVarP(&imageFlag, "image", "i", "", "Target all services matching a specific container image or registry alias (e.g. gar/app:v1.0)")

	return cmd
}
