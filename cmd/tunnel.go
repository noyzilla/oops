package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/tunnel"
	"github.com/spf13/cobra"
)

func newTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel [@remote] [services...]",
		Short: "Manage secure IAP SSH database tunnels to remote environments",
		Long: `Creates an isolated client tunnel container via Google Cloud IAP SSH to access remote databases (MySQL, Redis, PostgreSQL).
Connects without exposing remote server IP addresses and without exposing any ports on the local host machine.
Local applications and database GUIs connect directly via DNS: <service>.<remote>.tunnel.oops:<port> (e.g. mysql.b9tpe.tunnel.oops:3306, redis.b9tpe.tunnel.oops:6379)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if strings.HasPrefix(args[0], "@") {
				return runTunnelUp(cmd, args[0], args[1:])
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(newTunnelUpCmd())
	cmd.AddCommand(newTunnelDownCmd())
	cmd.AddCommand(newTunnelListCmd())
	cmd.AddCommand(newTunnelRestartCmd())

	return cmd
}

func newTunnelUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up @<remote> [service:port...]",
		Short: "Establish an IAP database tunnel to a remote server",
		Long: `Starts a local tunnel container connected to Oops network.
Default ports if omitted: mysql:3306 redis:6379.
Example: oops tunnel up @b9tpe
Example: oops tunnel up @b9tpe mysql:3306 redis:6379 pg:5432:5432`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelUp(cmd, args[0], args[1:])
		},
	}
}

func newTunnelDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down @<remote>",
		Short: "Tear down an active tunnel connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelDown(cmd, args[0])
		},
	}
}

func newTunnelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ps", "ls"},
		Short:   "List active tunnel connections and DNS endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelList(cmd)
		},
	}
}

func newTunnelRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart @<remote> [service:port...]",
		Short: "Re-resolve remote container IPs and restart tunnel",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = runTunnelDown(cmd, args[0])
			return runTunnelUp(cmd, args[0], args[1:])
		},
	}
}

func getDockerClient() (*client.Client, error) {
	dockerCli, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed connecting to Docker daemon: %w", err)
	}
	return dockerCli, nil
}

func runTunnelUp(cmd *cobra.Command, remoteTarget string, portSpecs []string) error {
	workDir := ResolveWorkDir(targetDir)

	cfg, err := tunnel.ResolveTunnelConfig(workDir, remoteTarget)
	if err != nil {
		return err
	}

	mappings, err := tunnel.ParsePortSpecs(portSpecs, cfg.Services)
	if err != nil {
		return err
	}

	dockerCli, err := getDockerClient()
	if err != nil {
		return err
	}
	defer dockerCli.Close()

	mgr := tunnel.NewManager(dockerCli, nil)

	cmd.Printf("==> Resolving remote container metadata on @%s over SSH...\n", cfg.RemoteName)
	ctx := context.Background()

	status, err := mgr.Up(ctx, cfg, mappings)
	if err != nil {
		return fmt.Errorf("failed starting tunnel: %w", err)
	}

	cmd.Printf("\n✓ Tunnel established to @%s\n", cfg.RemoteName)
	cmd.Printf("  Container : %s\n", status.Container)
	cmd.Printf("  Domain    : %s\n", status.Hostname)
	cmd.Println("  Endpoints :")
	for _, ep := range status.Endpoints {
		cmd.Printf("    - %s\n", ep)
	}
	cmd.Println("\nConnect directly from your workstation or GUI client using the endpoints above.")
	return nil
}

func runTunnelDown(cmd *cobra.Command, remoteTarget string) error {
	dockerCli, err := getDockerClient()
	if err != nil {
		return err
	}
	defer dockerCli.Close()

	cleanRemote := strings.TrimPrefix(strings.TrimSpace(remoteTarget), "@")
	mgr := tunnel.NewManager(dockerCli, nil)

	cmd.Printf("==> Stopping tunnel for @%s...\n", cleanRemote)
	if err := mgr.Down(context.Background(), cleanRemote); err != nil {
		return fmt.Errorf("failed stopping tunnel: %w", err)
	}

	cmd.Printf("✓ Tunnel for @%s stopped.\n", cleanRemote)
	return nil
}

func runTunnelList(cmd *cobra.Command) error {
	dockerCli, err := getDockerClient()
	if err != nil {
		return err
	}
	defer dockerCli.Close()

	mgr := tunnel.NewManager(dockerCli, nil)
	list, err := mgr.List(context.Background())
	if err != nil {
		return err
	}

	if len(list) == 0 {
		cmd.Println("No active tunnels. Start one with: oops tunnel @<remote>")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "REMOTE\tCONTAINER\tSTATUS\tENDPOINTS\tFORWARDED")

	for _, t := range list {
		endpointsStr := strings.Join(t.Endpoints, ", ")
		if endpointsStr == "" {
			endpointsStr = t.Hostname
		}
		fmt.Fprintf(w, "@%s\t%s\t%s\t%s\t%s\n", t.RemoteName, t.Container, t.Status, endpointsStr, t.Services)
	}
	return w.Flush()
}

