package cmd

import (
	"log"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "pull [targets...]",
		Short: "Pulls the latest images for stacks or targeted services",
		Long:  "Pulls latest container images across all stacks, a specific stack (/edge, /db), a group (@core, @all), or targeted services.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)

			var targets []docker.ResolvedTarget
			var err error

			if all && len(args) == 0 {
				orderedStacks, composeMap, err := docker.DiscoverStacks(workDir)
				if err != nil {
					return err
				}
				for _, s := range orderedStacks {
					cPath := composeMap[s]
					cfg, err := docker.ParseComposeFile(cPath)
					if err != nil {
						return err
					}
					for sName, srv := range cfg.Services {
						targets = append(targets, docker.ResolvedTarget{
							StackName:     s,
							ComposePath:   cPath,
							ServiceName:   sName,
							ContainerName: srv.ContainerName,
							Image:         srv.Image,
							Labels:        srv.ParsedLabels,
						})
					}
				}
			} else {
				targets, err = docker.ResolveTargets(workDir, args)
				if err != nil {
					return err
				}
			}

			if len(targets) == 0 {
				log.Println("No services found to pull.")
				return nil
			}

			// Group targets by stack to execute pull per compose file
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

			for _, stack := range orderedStacks {
				services := stackServiceMap[stack]
				composePath := stackComposeMap[stack]

				log.Printf("==> Pulling images for stack /%s (Services: %v)...", stack, services)
				cmdArgs := append([]string{"pull"}, services...)
				if err := orchestrator.RunComposeCommand(composePath, cmdArgs...); err != nil {
					return err
				}
			}

			log.Println("==> All targeted images pulled successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Pull images across all stacks regardless of default group")

	return cmd
}
