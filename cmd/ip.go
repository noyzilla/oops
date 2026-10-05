package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/spf13/cobra"
)

type containerIPInfo struct {
	IP        string
	Container string
	Service   string
	Networks  string
	Status    string
}

func newIPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ip [service]",
		Short: "Display IP address of a target service or list all container IPs",
		Long:  "Query and print the IP address of a running Docker container or compose service. If no service is specified, all active container IPs are listed.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runIPsList(cmd)
			}
			return runSingleIPLookup(cmd, args[0])
		},
	}
}


func runSingleIPLookup(cmd *cobra.Command, target string) error {
	workDir := ResolveWorkDir(targetDir)

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		dockerCli = nil
	}

	rec, err := dns.LookupRecord(context.Background(), workDir, target, dockerCli)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), rec.IP)
	return nil
}

func runIPsList(cmd *cobra.Command) error {
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed connecting to Docker daemon: %w", err)
	}

	ctx := context.Background()
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed listing Docker containers: %w", err)
	}

	var items []containerIPInfo

	for _, c := range containers {
		inspect, err := dockerCli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		cName := strings.TrimPrefix(inspect.Name, "/")
		serviceName := inspect.Config.Labels["com.docker.compose.service"]
		if serviceName == "" {
			serviceName = cName
		}

		status := inspect.State.Status
		if inspect.State.Health != nil && inspect.State.Health.Status != "" {
			status = fmt.Sprintf("%s (%s)", inspect.State.Status, inspect.State.Health.Status)
		}

		if inspect.NetworkSettings != nil && len(inspect.NetworkSettings.Networks) > 0 {
			var netNames []string
			for netName, netEndpoint := range inspect.NetworkSettings.Networks {
				netNames = append(netNames, netName)
				if netEndpoint.IPAddress != "" {
					items = append(items, containerIPInfo{
						IP:        netEndpoint.IPAddress,
						Container: cName,
						Service:   serviceName,
						Networks:  netName,
						Status:    status,
					})
				}
			}
		}
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No active container IP addresses found.")
		return nil
	}

	// Sort numerically by IP
	sort.Slice(items, func(i, j int) bool {
		if items[i].IP != items[j].IP {
			return dns.CompareIP(items[i].IP, items[j].IP)
		}
		return items[i].Container < items[j].Container
	})

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "IP\tCONTAINER\tSERVICE\tNETWORKS\tSTATUS")
	fmt.Fprintln(w, "----------------------------------------------------------------------------")
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", it.IP, it.Container, it.Service, it.Networks, it.Status)
	}
	w.Flush()

	return nil
}
