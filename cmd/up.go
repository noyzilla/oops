package cmd

import (
	"log"
	"time"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var delay string

	cmd := &cobra.Command{
		Use:   "up [targets...]",
		Short: "Starts stack, layer, or targeted services",
		Long:  "Starts stack layers or specific services. Target can be a layer (e.g. /apps, /db), scoped service (e.g. /db/mysql), service name (e.g. mysql), or wildcard (e.g. app..).",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			targets, err := docker.ResolveTargets(".", args)
			if err != nil {
				return err
			}

			// Group targets by layer to start layers cleanly
			layerServiceMap := make(map[string][]string)
			layerComposeMap := make(map[string]string)
			var orderedLayers []string

			for _, t := range targets {
				if _, exists := layerServiceMap[t.LayerName]; !exists {
					orderedLayers = append(orderedLayers, t.LayerName)
					layerComposeMap[t.LayerName] = t.ComposePath
				}
				layerServiceMap[t.LayerName] = append(layerServiceMap[t.LayerName], t.ServiceName)
			}

			for i, layer := range orderedLayers {
				services := layerServiceMap[layer]
				composePath := layerComposeMap[layer]

				log.Printf("==> Starting layer /%s (Services: %v)...", layer, services)
				cmdArgs := append([]string{"up", "-d"}, services...)
				if err := orchestrator.RunComposeCommand(composePath, cmdArgs...); err != nil {
					return err
				}

				if d > 0 && i < len(orderedLayers)-1 {
					log.Printf("Pausing %v before starting next layer...", d)
					time.Sleep(d)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive starts (e.g. 5s, 5)")

	return cmd
}
