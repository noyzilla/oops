package cmd

import (
	"context"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var delay string
	var imageFlag string

	cmd := &cobra.Command{
		Use:   "update [targets...]",
		Short: "Executes sequential rolling update with health check polling",
		Long:  "Executes sequential rolling update with pre-stop hooks, image pulling, and health check polling across targeted services, stack (/apps), group (@core), or container image (e.g. gar/app:v1.0 or -i image_name).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if imageFlag != "" {
				args = append(args, "img:"+imageFlag)
			}
			if len(args) == 0 {
				return cmd.Help()
			}

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

			return orch.Update(context.Background(), targets, d)
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive updates (e.g. 5s, 5)")
	cmd.Flags().StringVarP(&imageFlag, "image", "i", "", "Target all services matching a specific container image or registry alias (e.g. gar/app:v1.0)")

	return cmd
}
