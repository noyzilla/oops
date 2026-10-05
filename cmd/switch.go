package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newSwitchCmd() *cobra.Command {
	var delay string

	cmd := &cobra.Command{
		Use:   "switch <target>",
		Short: "Switches active profile/group: starts target and stops all other services",
		Long:  "Starts services in the specified group or stack (e.g. @core, @pg, /apps), and gracefully stops all other services not in the target.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			// 1. Resolve targets to start
			upTargets, err := docker.ResolveTargets(workDir, []string{target})
			if err != nil {
				return fmt.Errorf("failed to resolve target %s: %w", target, err)
			}

			// 2. Resolve targets to stop (all services except target)
			stopTargets, err := docker.ResolveTargetsWithExceptions(workDir, nil, []string{target})
			if err != nil {
				return fmt.Errorf("failed to resolve other services to stop: %w", err)
			}

			orch, err := orchestrator.New()
			if err != nil {
				return err
			}
			defer orch.Close()

			log.Printf("==> [Switch] Activating profile: %s (%d services)...", target, len(upTargets))
			if err := orch.Up(context.Background(), upTargets, d); err != nil {
				return fmt.Errorf("failed to start target profile: %w", err)
			}

			if len(stopTargets) > 0 {
				log.Printf("==> [Switch] Stopping %d inactive services not in %s...", len(stopTargets), target)
				if err := orch.Stop(context.Background(), stopTargets, d); err != nil {
					log.Printf("Warning: error during inactive services shutdown: %v", err)
				}
			}

			log.Printf("==> [Switch] Successfully switched to %s!", target)
			return nil
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration (e.g. 5s, 5)")

	return cmd
}
