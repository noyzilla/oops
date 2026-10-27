package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/tunnel"
	"github.com/spf13/cobra"
)

var (
	tunnelMeshMode bool
	tunnelPortMode bool
	tunnelMemory   string
)

func newTunnelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tunnel [@remote] [services...]",
		Short: "Manage secure IAP SSH database tunnels to remote environments",
		Long: `Creates an isolated client tunnel container via Google Cloud IAP SSH to access remote databases (MySQL, Redis, PostgreSQL).
Connects without exposing remote server IP addresses and without exposing any ports on the local host machine.
Local applications and database GUIs connect directly via DNS: <service>.<remote>:<port> (e.g. mysql.b9tpe:3306, redis.b9tpe:6379)`,
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

	cmd.PersistentFlags().BoolVar(&tunnelMeshMode, "mesh", false, "Use L3 user-space mesh routing mode over SOCKS5")
	cmd.PersistentFlags().BoolVar(&tunnelPortMode, "port", false, "Use standard L4 port forwarding mode")
	cmd.PersistentFlags().StringVar(&tunnelMemory, "memory", "", "Memory limit for tunnel container (e.g. 128m, 256m, default 256m)")

	cmd.AddCommand(newTunnelUpCmd())
	cmd.AddCommand(newTunnelDownCmd())
	cmd.AddCommand(newTunnelListCmd())
	cmd.AddCommand(newTunnelRestartCmd())

	return cmd
}

func newTunnelUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up @<remote> [service:port...]",
		Short: "Establish an IAP database tunnel to a remote server",
		Long: `Starts a local tunnel container connected to Oops network.
Default ports if omitted: mysql:3306 redis:6379.
Example: oops tunnel up @b9tpe
Example: oops tunnel up @b9tpe --mesh
Example: oops tunnel up @b9tpe mysql:3306 redis:6379 pg:5432:5432`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnelUp(cmd, args[0], args[1:])
		},
	}

	cmd.Flags().BoolVar(&tunnelMeshMode, "mesh", false, "Use L3 user-space mesh routing mode over SOCKS5")
	cmd.Flags().BoolVar(&tunnelPortMode, "port", false, "Use standard L4 port forwarding mode")
	cmd.Flags().StringVar(&tunnelMemory, "memory", "", "Memory limit for tunnel container (e.g. 128m, 256m, default 256m)")

	return cmd
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

	if tunnelMeshMode {
		cfg.Mode = "mesh"
	} else if tunnelPortMode {
		cfg.Mode = "port"
	}

	if tunnelMemory != "" {
		bytes, err := units.RAMInBytes(tunnelMemory)
		if err != nil {
			return fmt.Errorf("invalid memory limit %q: %w", tunnelMemory, err)
		}
		cfg.MemoryLimit = bytes
		cfg.MemoryReservation = bytes / 2
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

	workDir := ResolveWorkDir(targetDir)
	return formatTunnelList(cmd.OutOrStdout(), list, workDir)
}

func formatTunnelList(out io.Writer, list []tunnel.TunnelStatus, workDir string) error {
	w := tabwriter.NewWriter(out, 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "REMOTE\tCONTAINER\tSTATUS\tENDPOINTS")

	serviceToScoped := make(map[string]string)
	resolveService := func(svc string) string {
		if s, ok := serviceToScoped[svc]; ok {
			return s
		}
		if workDir != "" {
			if targets, err := docker.ResolveTargets(workDir, []string{svc}); err == nil && len(targets) > 0 {
				res := fmt.Sprintf("/%s/%s", targets[0].StackName, targets[0].ServiceName)
				serviceToScoped[svc] = res
				return res
			}
		}
		serviceToScoped[svc] = ""
		return ""
	}

	for _, t := range list {
		endpoints := t.Endpoints
		if len(endpoints) == 0 && t.Hostname != "" {
			endpoints = []string{t.Hostname}
		}

		if len(endpoints) == 0 {
			fmt.Fprintf(w, "@%s\t%s\t%s\t-\n", t.RemoteName, t.Container, t.Status)
			continue
		}

		for i, ep := range endpoints {
			svcName := strings.Split(ep, ".")[0]
			if idx := strings.Index(svcName, ":"); idx != -1 {
				svcName = svcName[:idx]
			}
			scoped := resolveService(svcName)
			displayEp := ep
			if scoped != "" {
				displayEp = fmt.Sprintf("%s (%s)", ep, scoped)
			}

			if i == 0 {
				fmt.Fprintf(w, "@%s\t%s\t%s\t%s\n", t.RemoteName, t.Container, t.Status, displayEp)
			} else {
				fmt.Fprintf(w, "\t\t\t%s\n", displayEp)
			}
		}
	}
	return w.Flush()
}

