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
		Short: "Starts stack or targeted services",
		Long:  "Starts stack or specific services. Target can be a whole stack (e.g. /apps, /db), scoped service (e.g. /db/mysql), service name (e.g. mysql), or wildcard (e.g. app..).",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			targets, err := docker.ResolveTargets(workDir, args)
			if err != nil {
				return err
			}

			// Group targets by stack to start stacks cleanly
			stackServiceMap := make(map[string][]string)
			stackComposeMap := make(map[string]string)
			var orderedStacks []string

			for _, t := range targets {
				if _, exists := stackServiceMap[t.StackName]; !exists {
					orderedStacks = append(orderedStacks, t.StackName)
					stackComposeMap[t.StackName] = t.ComposePath
				}
				stackServiceMap[t.StackName] = append(stackServiceMap[t.StackName], t.ServiceName)
			}

			for i, stack := range orderedStacks {
				services := stackServiceMap[stack]
				composePath := stackComposeMap[stack]

				log.Printf("==> Starting stack /%s (Services: %v)...", stack, services)
				cmdArgs := append([]string{"up", "-d"}, services...)
				if err := orchestrator.RunComposeCommand(composePath, cmdArgs...); err != nil {
					return err
				}

				if d > 0 && i < len(orderedStacks)-1 {
					log.Printf("Pausing %v before starting next stack...", d)
					time.Sleep(d)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive starts (e.g. 5s, 5)")

	return cmd
}
