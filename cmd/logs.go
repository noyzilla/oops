package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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
	if err := ValidateActiveBox(workDir); err != nil {
		return err
	}

	// 1. Direct single-container lookup (fastest & most reliable without compose .env interpolation issues)
	if len(args) == 1 {
		targetName := args[0]
		dockerCli, cliErr := docker.NewClient()
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

	// 2. Resolve via compose stacks (for multi-targets, default x-oops-stack, or stack paths like /edge, /db)
	targets, err := docker.ResolveTargets(workDir, args)
	if err == nil && len(targets) > 0 {
		composeArgs, services := buildComposeLogsArgs(workDir, targets, tail, follow)

		c := exec.Command("docker", composeArgs...)
		c.Stdin = os.Stdin
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		if err := c.Run(); err != nil {
			// Fallback to direct docker logs on each container/service with proper stdout/stderr binding
			for i, svc := range services {
				containerRef := svc
				if i < len(targets) && targets[i].ContainerName != "" {
					containerRef = targets[i].ContainerName
				}
				fbArgs := []string{"logs", "--tail", strconv.Itoa(tail)}
				if follow && len(services) == 1 {
					fbArgs = append(fbArgs, "-f")
				}
				fbArgs = append(fbArgs, containerRef)
				fbCmd := exec.Command("docker", fbArgs...)
				fbCmd.Stdin = os.Stdin
				fbCmd.Stdout = cmd.OutOrStdout()
				fbCmd.Stderr = cmd.ErrOrStderr()
				_ = fbCmd.Run()
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

// buildComposeLogsArgs constructs the unified 'docker compose [--env-file .env] -f ... logs' arguments
// across all resolved targets while preserving Tail-Chaining for compose.local.yaml.
func buildComposeLogsArgs(workDir string, targets []docker.ResolvedTarget, tail int, follow bool) ([]string, []string) {
	var composeFiles []string
	seenFile := make(map[string]bool)
	var globalLocalFile string

	for _, t := range targets {
		files := t.ComposeFiles
		if len(files) == 0 && t.ComposePath != "" {
			files = []string{t.ComposePath}
		}
		for _, f := range files {
			if f == "" {
				continue
			}
			if filepath.Base(f) == "compose.local.yaml" {
				globalLocalFile = f
				continue
			}
			if !seenFile[f] {
				seenFile[f] = true
				composeFiles = append(composeFiles, f)
			}
		}
	}
	if globalLocalFile != "" && !seenFile[globalLocalFile] {
		composeFiles = append(composeFiles, globalLocalFile)
	}

	var services []string
	seenSvc := make(map[string]bool)
	for _, t := range targets {
		if t.ServiceName != "" && !seenSvc[t.ServiceName] {
			seenSvc[t.ServiceName] = true
			services = append(services, t.ServiceName)
		}
	}

	composeArgs := []string{"compose"}
	candidate := filepath.Join(workDir, ".env")
	if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
		composeArgs = append(composeArgs, "--env-file", candidate)
	}
	for _, f := range composeFiles {
		composeArgs = append(composeArgs, "-f", f)
	}
	composeArgs = append(composeArgs, "logs", "--tail", strconv.Itoa(tail))
	if follow {
		composeArgs = append(composeArgs, "-f")
	}
	composeArgs = append(composeArgs, services...)

	return composeArgs, services
}
