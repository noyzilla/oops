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
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

type containerStatusItem struct {
	Stack     string
	Service   string
	Container string
	Status    string
	IP        string
	Ports     string
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [targets...]",
		Short: "Displays formatted status of containers, health, IPs, and ports",
		Long:  "Inspects and displays formatted status for active Docker containers across Oopsbox compose stacks or targeted services.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args)
		},
	}

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	workDir := ResolveWorkDir(targetDir)

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed connecting to Docker daemon: %w", err)
	}

	ctx := context.Background()
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed listing Docker containers: %w", err)
	}

	// Filter targets if args provided
	var targetMap map[string]bool
	if len(args) > 0 {
		resolved, err := docker.ResolveTargets(workDir, args)
		if err == nil && len(resolved) > 0 {
			targetMap = make(map[string]bool)
			for _, r := range resolved {
				targetMap[r.ContainerName] = true
				targetMap[r.ServiceName] = true
			}
		}
	}

	var items []containerStatusItem

	for _, c := range containers {
		inspect, err := dockerCli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		cName := strings.TrimPrefix(inspect.Name, "/")
		svcName := inspect.Config.Labels["com.docker.compose.service"]
		if svcName == "" {
			svcName = cName
		}

		stackName := inspect.Config.Labels["com.docker.compose.project"]
		if stackName == "" {
			stackName = "default"
		}

		// Target filtering check
		if targetMap != nil {
			matched := targetMap[cName] || targetMap[svcName] || targetMap[stackName]
			if !matched {
				for _, arg := range args {
					argClean := strings.TrimPrefix(arg, "/")
					if strings.EqualFold(cName, argClean) || strings.EqualFold(svcName, argClean) || strings.EqualFold(stackName, argClean) || strings.Contains(strings.ToLower(cName), strings.ToLower(argClean)) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Status calculation
		status := inspect.State.Status
		if inspect.State.Health != nil && inspect.State.Health.Status != "" {
			status = fmt.Sprintf("%s (%s)", inspect.State.Status, inspect.State.Health.Status)
		} else if inspect.State.ExitCode != 0 {
			status = fmt.Sprintf("%s (%d)", inspect.State.Status, inspect.State.ExitCode)
		}

		// Extract primary IP
		ip := dns.ExtractContainerIP(&inspect)
		ipStr := "-"
		if ip != nil {
			ipStr = ip.String()
		}

		// Extract ports
		ports := formatContainerPorts(&inspect)

		items = append(items, containerStatusItem{
			Stack:     stackName,
			Service:   svcName,
			Container: cName,
			Status:    status,
			IP:        ipStr,
			Ports:     ports,
		})
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No active containers found matching specified targets.")
		return nil
	}

	// Sort items by Stack, then Service
	sort.Slice(items, func(i, j int) bool {
		if items[i].Stack != items[j].Stack {
			return items[i].Stack < items[j].Stack
		}
		return items[i].Service < items[j].Service
	})

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STACK\tSERVICE\tSTATUS\tIP\tPORTS (*=all, #=local)")
	fmt.Fprintln(w, "----------------------------------------------------------------------------------------")
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", it.Stack, it.Service, it.Status, it.IP, it.Ports)
	}
	w.Flush()

	return nil
}

type boundGroupKey struct {
	prefix        string
	containerPort int
	proto         string
}

type boundGroupValue struct {
	hostPorts map[int]bool
}

type renderedBoundEntry struct {
	containerPort int
	firstHostPort int
	display       string
}

type internalPortEntry struct {
	containerPort int
	proto         string
	display       string
}

func formatContainerPorts(inspect *container.InspectResponse) string {
	if inspect == nil || inspect.NetworkSettings == nil || len(inspect.NetworkSettings.Ports) == 0 {
		return "-"
	}

	boundGroups := make(map[boundGroupKey]*boundGroupValue)
	var internalEntries []internalPortEntry

	for pKey, pBindings := range inspect.NetworkSettings.Ports {
		cPortNum := 0
		fmt.Sscanf(pKey.Port(), "%d", &cPortNum)
		proto := strings.ToLower(pKey.Proto())
		if proto == "" {
			proto = "tcp"
		}

		hasBinding := false
		if len(pBindings) > 0 {
			for _, b := range pBindings {
				if b.HostPort != "" {
					hasBinding = true
					hPortNum := 0
					fmt.Sscanf(b.HostPort, "%d", &hPortNum)

					prefix := "*:"
					if b.HostIP == "127.0.0.1" || b.HostIP == "::1" || b.HostIP == "localhost" {
						prefix = "#:"
					} else if b.HostIP != "" && b.HostIP != "0.0.0.0" && b.HostIP != "::" {
						prefix = fmt.Sprintf("%s:", b.HostIP)
					}

					key := boundGroupKey{
						prefix:        prefix,
						containerPort: cPortNum,
						proto:         proto,
					}
					if boundGroups[key] == nil {
						boundGroups[key] = &boundGroupValue{hostPorts: make(map[int]bool)}
					}
					boundGroups[key].hostPorts[hPortNum] = true
				}
			}
		}

		if !hasBinding && pKey.Port() != "" {
			display := pKey.Port()
			if proto != "tcp" {
				display = fmt.Sprintf("%s/%s", pKey.Port(), proto)
			}
			internalEntries = append(internalEntries, internalPortEntry{
				containerPort: cPortNum,
				proto:         proto,
				display:       display,
			})
		}
	}

	var renderedBounds []renderedBoundEntry
	for key, val := range boundGroups {
		var hPorts []int
		for hp := range val.hostPorts {
			hPorts = append(hPorts, hp)
		}
		sort.Ints(hPorts)

		var hStrs []string
		for _, hp := range hPorts {
			hStrs = append(hStrs, fmt.Sprintf("%d", hp))
		}

		protoSuffix := ""
		if key.proto != "tcp" {
			protoSuffix = fmt.Sprintf("/%s", key.proto)
		}

		var display string
		if len(hPorts) == 1 {
			if hPorts[0] == key.containerPort {
				display = fmt.Sprintf("%s%d%s", key.prefix, hPorts[0], protoSuffix)
			} else {
				display = fmt.Sprintf("%s%d->%d%s", key.prefix, hPorts[0], key.containerPort, protoSuffix)
			}
		} else {
			hPart := fmt.Sprintf("[%s]", strings.Join(hStrs, ","))
			display = fmt.Sprintf("%s%s->%d%s", key.prefix, hPart, key.containerPort, protoSuffix)
		}

		firstHP := 0
		if len(hPorts) > 0 {
			firstHP = hPorts[0]
		}

		renderedBounds = append(renderedBounds, renderedBoundEntry{
			containerPort: key.containerPort,
			firstHostPort: firstHP,
			display:       display,
		})
	}

	sort.Slice(renderedBounds, func(i, j int) bool {
		if renderedBounds[i].firstHostPort != renderedBounds[j].firstHostPort {
			return renderedBounds[i].firstHostPort < renderedBounds[j].firstHostPort
		}
		return renderedBounds[i].containerPort < renderedBounds[j].containerPort
	})

	sort.Slice(internalEntries, func(i, j int) bool {
		if internalEntries[i].containerPort != internalEntries[j].containerPort {
			return internalEntries[i].containerPort < internalEntries[j].containerPort
		}
		return internalEntries[i].proto < internalEntries[j].proto
	})

	var allPortStrs []string
	for _, rb := range renderedBounds {
		allPortStrs = append(allPortStrs, rb.display)
	}

	for _, ie := range internalEntries {
		allPortStrs = append(allPortStrs, ie.display)
	}

	if len(allPortStrs) > 0 {
		return strings.Join(allPortStrs, ", ")
	}

	return "-"
}
