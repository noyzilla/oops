package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var tail int
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs [targets...]",
		Short: "Tails logs for target compose services or containers",
		Long:  "Fetches and streams logs from Docker Compose services or running containers matching the specified target.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, args, tail, follow)
		},
	}

	cmd.Flags().IntVarP(&tail, "tail", "t", 50, "Number of lines to show from the end of the logs")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}

func runLogs(cmd *cobra.Command, args []string, tail int, follow bool) error {
	workDir := ResolveWorkDir(targetDir)

	// 1. Direct single-container lookup (fastest & most reliable without compose .env interpolation issues)
	if len(args) == 1 {
		targetName := args[0]
		dockerCli, cliErr := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if cliErr == nil && dockerCli != nil {
			rec, recErr := dns.LookupRecord(context.Background(), workDir, targetName, dockerCli)
			if recErr == nil && rec.Container != "" {
				dockerArgs := []string{"logs", "--tail", strconv.Itoa(tail)}
				if follow {
					dockerArgs = append(dockerArgs, "-f")
				}
				dockerArgs = append(dockerArgs, rec.Container)

				c := exec.Command("docker", dockerArgs...)
				c.Stdin = os.Stdin
				c.Stdout = cmd.OutOrStdout()
				c.Stderr = cmd.ErrOrStderr()
				return c.Run()
			}
		}
	}

	// 2. Resolve via compose stacks (for multi-targets or stack paths like /edge, /db)
	targets, err := docker.ResolveTargets(workDir, args)
	if err == nil && len(targets) > 0 {
		// Group targets by compose file
		byCompose := make(map[string][]string)
		for _, t := range targets {
			byCompose[t.ComposePath] = append(byCompose[t.ComposePath], t.ServiceName)
		}

		// Find .env file in workspace
		var envFile string
		for _, candidate := range []string{filepath.Join(workDir, ".env"), filepath.Join(workDir, "stacks", ".env")} {
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				envFile = candidate
				break
			}
		}

		for composePath, services := range byCompose {
			composeArgs := []string{"compose"}
			if envFile != "" {
				composeArgs = append(composeArgs, "--env-file", envFile)
			}
			composeArgs = append(composeArgs, "-f", composePath, "logs", "--tail", strconv.Itoa(tail))
			if follow {
				composeArgs = append(composeArgs, "-f")
			}
			composeArgs = append(composeArgs, services...)

			c := exec.Command("docker", composeArgs...)
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				// Fallback to docker logs on each container
				for _, svc := range services {
					_ = exec.Command("docker", "logs", "--tail", strconv.Itoa(tail), svc).Run()
				}
			}
		}
		return nil
	}

	// 3. Fallback direct docker logs on argument string
	if len(args) > 0 {
		dockerArgs := []string{"logs", "--tail", strconv.Itoa(tail)}
		if follow {
			dockerArgs = append(dockerArgs, "-f")
		}
		dockerArgs = append(dockerArgs, args[0])

		c := exec.Command("docker", dockerArgs...)
		c.Stdin = os.Stdin
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		return c.Run()
	}

	if err != nil {
		return err
	}
	return fmt.Errorf("no target services or containers found to fetch logs")
}
