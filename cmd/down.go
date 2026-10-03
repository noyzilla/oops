package cmd

import (
	"context"
	"log"
	"time"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var delay string

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Tears down all layers in reverse dependency order",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			orderedLayers, composeMap, err := docker.DiscoverLayers(".")
			if err != nil {
				return err
			}

			// Reverse order for teardown (apps -> utils -> db -> edge)
			var reverseLayers []string
			for i := len(orderedLayers) - 1; i >= 0; i-- {
				reverseLayers = append(reverseLayers, orderedLayers[i])
			}

			orch, err := orchestrator.New()
			if err == nil {
				defer orch.Close()
				// Run pre-stop hooks on all running containers
				allTargets, err := docker.ResolveTargets(".", nil)
				if err == nil {
					for _, t := range allTargets {
						cID, err := orch.FindContainerID(context.Background(), t)
						if err == nil && cID != "" {
							stopCmd := t.Labels["oops.stop.cmd"]
							stopTimeout := orchestrator.GetStopTimeout(t.Labels)
							if stopCmd != "" {
								_ = orchestrator.ExecutePreStopHook(context.Background(), nil, cID, stopCmd, stopTimeout)
							}
						}
					}
				}
			}

			for i, layer := range reverseLayers {
				composePath := composeMap[layer]
				log.Printf("==> Tearing down layer /%s...", layer)
				if err := orchestrator.RunComposeCommand(composePath, "down"); err != nil {
					log.Printf("Warning: down failed for /%s: %v", layer, err)
				}

				if d > 0 && i < len(reverseLayers)-1 {
					log.Printf("Pausing %v before tearing down next layer...", d)
					time.Sleep(d)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive stops (e.g. 5s, 5)")

	return cmd
}
