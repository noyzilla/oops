package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var delay string
	var wipeAll bool
	var autoConfirm bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Tears down all stacks in reverse dependency order (or wipes all containers with --wipe-all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if wipeAll {
				ctx := context.Background()
				count, err := docker.CountAllContainers(ctx)
				if err != nil {
					return err
				}
				if count == 0 {
					log.Println("No containers found on Docker daemon. Docker host is already clean.")
					return nil
				}

				if !autoConfirm {
					fmt.Printf("WARNING: This will stop and remove ALL %d containers on this Docker daemon.\n", count)
					fmt.Print("Are you sure you want to proceed? [y/N]: ")
					reader := bufio.NewReader(os.Stdin)
					input, _ := reader.ReadString('\n')
					ans := strings.TrimSpace(strings.ToLower(input))
					if ans != "y" && ans != "yes" {
						log.Println("Wipe cancelled.")
						return nil
					}
				}

				log.Printf("==> Wiping all %d containers on Docker daemon...", count)
				removed, err := docker.WipeAllContainers(ctx)
				if err != nil {
					return err
				}
				log.Printf("✓ Successfully removed %d containers. Docker host is clean.", removed)
				return nil
			}

			d, err := orchestrator.ParseDelay(delay)
			if err != nil {
				return err
			}

			workDir := ResolveWorkDir(targetDir)
			orderedStacks, composeMap, err := docker.DiscoverStacks(workDir)
			if err != nil {
				return err
			}

			// Reverse order for teardown (apps -> tool -> db -> edge)
			var reverseStacks []string
			for i := len(orderedStacks) - 1; i >= 0; i-- {
				reverseStacks = append(reverseStacks, orderedStacks[i])
			}

			orch, err := orchestrator.New()
			if err == nil {
				defer orch.Close()
				// Run pre-stop hooks on all running containers
				allTargets, err := docker.ResolveTargets(workDir, nil)
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

			for i, stack := range reverseStacks {
				composePath := composeMap[stack]
				log.Printf("==> Tearing down stack /%s...", stack)
				if err := orchestrator.RunComposeCommand(composePath, "down"); err != nil {
					log.Printf("Warning: down failed for /%s: %v", stack, err)
				}

				if d > 0 && i < len(reverseStacks)-1 {
					log.Printf("Pausing %v before tearing down next stack...", d)
					time.Sleep(d)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&delay, "delay", "d", "0s", "Inter-service delay duration between consecutive stops (e.g. 5s, 5)")
	cmd.Flags().BoolVar(&wipeAll, "wipe-all", false, "Forcefully stop and remove all containers on the Docker daemon")
	cmd.Flags().BoolVarP(&autoConfirm, "yes", "y", false, "Automatic yes to prompts (for non-interactive execution)")

	return cmd
}
