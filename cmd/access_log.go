package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/term"
	"github.com/noyzilla/oops/internal/accesslog"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

func newAccessLogCmd() *cobra.Command {
	var (
		tail     int
		follow   bool
		noFollow bool
		level    string
		status   string
		method   string
		host     string
		domain   string
		path     string
		scheme   string
		client   string
		ip       string
		latency  string
		all      bool
	)

	cmd := &cobra.Command{
		Use:   "access-log [container]",
		Short: "Streams and filters normalized HTTP access logs from edge proxy containers",
		Long: `Streams and filters normalized HTTP access logs from reverse proxy containers (e.g. Caddy, Nginx).
Formats logs into an ergonomic, colorized single-line table with regex filtering support.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			actualHost := host
			if actualHost == "" {
				actualHost = domain
			}
			actualClient := client
			if actualClient == "" {
				actualClient = ip
			}

			filterFlags := accesslog.FilterFlags{
				Level:   level,
				Status:  status,
				Method:  method,
				Host:    actualHost,
				Path:    path,
				Scheme:  scheme,
				Client:  actualClient,
				Latency: latency,
				All:     all,
			}

			target := ""
			if len(args) > 0 {
				target = args[0]
			}

			isFollow := follow && !noFollow
			return runAccessLog(cmd, target, tail, isFollow, filterFlags)
		},
	}

	cmd.Flags().IntVarP(&tail, "tail", "t", 100, "Number of lines to show from the end of the logs")
	cmd.Flags().BoolVarP(&follow, "follow", "f", true, "Follow log output continuously")
	cmd.Flags().BoolVarP(&noFollow, "no-follow", "d", false, "Do not follow log output (dump historical lines and exit)")
	cmd.Flags().StringVar(&level, "level", "", "Filter by log level regex (e.g. error, error|warn)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by HTTP status code regex (e.g. ^5.., 404)")
	cmd.Flags().StringVar(&method, "method", "", "Filter by HTTP method regex (e.g. POST|PUT, GET)")
	cmd.Flags().StringVar(&host, "host", "", "Filter by HTTP host/domain regex (e.g. .*.oops)")
	cmd.Flags().StringVar(&domain, "domain", "", "Alias for --host")
	cmd.Flags().StringVar(&path, "path", "", "Filter by URI path regex (e.g. ^/api)")
	cmd.Flags().StringVar(&scheme, "scheme", "", "Filter by protocol scheme regex (e.g. https?)")
	cmd.Flags().StringVar(&client, "client", "", "Filter by client IP regex")
	cmd.Flags().StringVar(&ip, "ip", "", "Alias for --client")
	cmd.Flags().StringVar(&latency, "latency", "", "Filter by latency duration (e.g. >10ms, <5ms, >10ms,<100ms)")
	cmd.Flags().BoolVar(&all, "all", false, "Include proxy internal/system logs in output")

	return cmd
}

func runAccessLog(cmd *cobra.Command, target string, tail int, follow bool, flags accesslog.FilterFlags) error {
	workDir := ResolveWorkDir(targetDir)
	if err := ValidateActiveBox(workDir); err != nil {
		return err
	}

	filterCriteria, err := accesslog.ParseFilterCriteria(flags)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	containerName, err := resolveProxyContainer(ctx, workDir, target)
	if err != nil {
		return err
	}

	decoder := accesslog.NewAutoDecoder(containerName)

	isTTY := false
	if file, ok := cmd.OutOrStdout().(*os.File); ok {
		isTTY = term.IsTerminal(file.Fd())
	}
	formatter := accesslog.NewFormatter(isTTY)

	// Print table header
	cmd.Println(formatter.FormatHeader())

	// Build docker logs arguments
	dockerArgs := []string{"logs", "--tail", strconv.Itoa(tail)}
	if follow {
		dockerArgs = append(dockerArgs, "-f")
	}
	dockerArgs = append(dockerArgs, containerName)

	c := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Pipe both stdout and stderr together to capture logs regardless of where the proxy writes
	pr, pw := io.Pipe()
	c.Stdout = pw
	c.Stderr = pw

	// Run docker command in background
	errCh := make(chan error, 1)
	go func() {
		err := c.Run()
		_ = pw.Close()
		errCh <- err
	}()

	scanner := bufio.NewScanner(pr)
	// Allocate 1MB max line buffer for large JSON entries
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		rec, decErr := decoder.Decode(line)
		if decErr != nil || rec == nil {
			continue
		}

		if filterCriteria.Matches(rec) {
			cmd.Println(formatter.FormatRecord(rec))
		}
	}

	if scanErr := scanner.Err(); scanErr != nil && scanErr != io.EOF {
		return scanErr
	}

	cmdErr := <-errCh
	if cmdErr != nil && ctx.Err() == nil {
		return cmdErr
	}
	return nil
}

func resolveProxyContainer(ctx context.Context, workDir string, target string) (string, error) {
	dockerCli, err := docker.NewClient()
	if err != nil {
		if target != "" {
			return target, nil
		}
		return "", fmt.Errorf("failed to initialize docker client: %w", err)
	}
	defer dockerCli.Close()

	// 1. If explicit target argument given, lookup via DNS or container name
	if target != "" {
		rec, recErr := dns.LookupRecord(ctx, workDir, target, dockerCli)
		if recErr == nil && rec.Container != "" {
			return rec.Container, nil
		}
		return target, nil
	}

	// 2. Auto-discovery among running containers
	containers, listErr := dockerCli.ContainerList(ctx, container.ListOptions{})
	if listErr != nil {
		return "", fmt.Errorf("failed to list docker containers: %w", listErr)
	}

	// Candidate priority:
	// a. exact name caddy-proxy or caddy
	for _, c := range containers {
		for _, name := range c.Names {
			trimmed := strings.TrimPrefix(name, "/")
			if trimmed == "caddy-proxy" || trimmed == "caddy" {
				return trimmed, nil
			}
		}
	}

	// b. service name caddy-proxy or caddy or in stack /edge
	for _, c := range containers {
		svc := c.Labels["com.docker.compose.service"]
		if svc == "caddy-proxy" || svc == "caddy" {
			if len(c.Names) > 0 {
				return strings.TrimPrefix(c.Names[0], "/"), nil
			}
		}
	}

	// c. any container whose name or image contains "caddy"
	for _, c := range containers {
		for _, name := range c.Names {
			trimmed := strings.TrimPrefix(name, "/")
			if strings.Contains(strings.ToLower(trimmed), "caddy") {
				return trimmed, nil
			}
		}
		if strings.Contains(strings.ToLower(c.Image), "caddy") {
			if len(c.Names) > 0 {
				return strings.TrimPrefix(c.Names[0], "/"), nil
			}
		}
	}

	// d. any container whose name or image contains "nginx"
	for _, c := range containers {
		for _, name := range c.Names {
			trimmed := strings.TrimPrefix(name, "/")
			if strings.Contains(strings.ToLower(trimmed), "nginx") {
				return trimmed, nil
			}
		}
		if strings.Contains(strings.ToLower(c.Image), "nginx") {
			if len(c.Names) > 0 {
				return strings.TrimPrefix(c.Names[0], "/"), nil
			}
		}
	}

	return "", fmt.Errorf("no running edge proxy container found (searched for caddy-proxy, caddy, nginx). Specify target explicitly: oops access-log <container>")
}

