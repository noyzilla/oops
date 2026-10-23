package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/api/types/container"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/spf13/cobra"
)

type containerStatusItem struct {
	ContainerID  string
	Stack        string
	Service      string
	StackService string
	Container    string
	Image        string
	Status       string
	IP           string
	Ports        string
}

func newPsCmd() *cobra.Command {
	var fullImage bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "ps [targets...]",
		Short: "Displays formatted status of containers, health, IPs, and ports",
		Long:  "Inspects and displays formatted status for Docker containers across Oopsbox compose stacks or targeted services. Use -a/--all to show uncreated services.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPs(cmd, args, fullImage, showAll)
		},
	}

	cmd.Flags().BoolVar(&fullImage, "full", false, "Display full container image path including registry and repository (-fl)")
	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all services including uncreated/missing services from compose files")

	return cmd
}

func shortContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func formatContainerImage(image string, full bool) string {
	if full || image == "" {
		return image
	}
	if idx := strings.LastIndex(image, "/"); idx != -1 && idx < len(image)-1 {
		return image[idx+1:]
	}
	return image
}

func extractContainerStackName(labels map[string]string) string {
	if cFiles, ok := labels["com.docker.compose.project.config_files"]; ok && cFiles != "" {
		var matchedStack string
		for _, part := range strings.Split(cFiles, ",") {
			base := filepath.Base(strings.TrimSpace(part))
			if strings.HasPrefix(base, "compose.") && strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".local.yaml") {
				stack := strings.TrimSuffix(strings.TrimPrefix(base, "compose."), ".yaml")
				if stack != "" {
					matchedStack = stack
				}
			}
		}
		if matchedStack != "" {
			return matchedStack
		}
	}
	if proj := labels["com.docker.compose.project"]; proj != "" {
		return proj
	}
	return "default"
}

func formatStackService(stackName, serviceName string) string {
	if stackName == "" || stackName == "." {
		return "/" + serviceName
	}
	return fmt.Sprintf("/%s/%s", strings.TrimPrefix(stackName, "/"), serviceName)
}

func renderStatusTable(out io.Writer, items []containerStatusItem) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CONTAINER ID\tSTACK\tSERVICE\tIMAGE\tSTATUS\tIP\tPORTS (*=all, #=local)")
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			it.ContainerID,
			it.Stack,
			it.Service,
			it.Image,
			it.Status,
			it.IP,
			it.Ports,
		)
	}
	w.Flush()
}

// resolveServiceImageAndPorts resolves image and ports traversing extends hierarchy if necessary
func resolveServiceImageAndPorts(workDir string, composeDir string, currentCfg *docker.ComposeConfig, svc docker.ComposeService, depth int) (string, []string) {
	image := svc.Image
	ports := svc.Ports

	if (image != "" && len(ports) > 0) || depth > 5 {
		return image, ports
	}

	if svc.ParsedExtends != nil && svc.ParsedExtends.Service != "" {
		targetFile := composeDir
		if svc.ParsedExtends.File != "" {
			if filepath.IsAbs(svc.ParsedExtends.File) {
				targetFile = svc.ParsedExtends.File
			} else {
				// Try relative to composeDir first, then workDir
				cand := filepath.Join(filepath.Dir(composeDir), svc.ParsedExtends.File)
				if _, err := os.Stat(cand); err == nil {
					targetFile = cand
				} else {
					targetFile = filepath.Join(workDir, svc.ParsedExtends.File)
				}
			}
		}

		var targetCfg *docker.ComposeConfig
		if targetFile == composeDir && currentCfg != nil {
			targetCfg = currentCfg
		} else {
			var err error
			targetCfg, err = docker.ParseComposeFile(targetFile)
			if err != nil {
				targetCfg = nil
			}
		}

		if targetCfg != nil {
			if baseSvc, ok := targetCfg.Services[svc.ParsedExtends.Service]; ok {
				baseImg, basePorts := resolveServiceImageAndPorts(workDir, targetFile, targetCfg, baseSvc, depth+1)
				if image == "" {
					image = baseImg
				}
				if len(ports) == 0 {
					ports = basePorts
				}
			}
		}
	}

	return image, ports
}

func runPs(cmd *cobra.Command, args []string, fullImage bool, showAll bool) error {
	workDir := ResolveWorkDir(targetDir)
	if err := ValidateActiveBox(workDir); err != nil {
		return err
	}

	dockerCli, err := docker.NewClient()
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
				if r.StackName != "" {
					targetMap[fmt.Sprintf("/%s/%s", r.StackName, r.ServiceName)] = true
				}
			}
		}
	}

	var items []containerStatusItem
	existingServices := make(map[string]bool)

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

		stackName := extractContainerStackName(inspect.Config.Labels)
		stackService := formatStackService(stackName, svcName)
		projectLabel := inspect.Config.Labels["com.docker.compose.project"]

		// Target filtering check
		if targetMap != nil {
			matched := targetMap[cName] || targetMap[svcName] || targetMap[stackName] || targetMap[stackService] || targetMap[projectLabel]
			if !matched {
				for _, arg := range args {
					argClean := strings.TrimPrefix(arg, "/")
					if strings.EqualFold(cName, argClean) ||
						strings.EqualFold(svcName, argClean) ||
						strings.EqualFold(stackName, argClean) ||
						strings.EqualFold(strings.TrimPrefix(stackService, "/"), argClean) ||
						strings.Contains(strings.ToLower(cName), strings.ToLower(argClean)) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Track existing service by stack+service and containerName
		existingServices[fmt.Sprintf("%s/%s", stackName, svcName)] = true
		existingServices[cName] = true
		existingServices[svcName] = true

		// Status calculation (prefer Docker summary status which includes uptime and health, e.g. "Up 2 hours (healthy)")
		status := strings.TrimSpace(c.Status)
		if status == "" {
			status = inspect.State.Status
			if inspect.State.Health != nil && inspect.State.Health.Status != "" {
				status = fmt.Sprintf("%s (%s)", inspect.State.Status, inspect.State.Health.Status)
			} else if inspect.State.ExitCode != 0 {
				status = fmt.Sprintf("%s (%d)", inspect.State.Status, inspect.State.ExitCode)
			}
		}

		rawImage := c.Image
		if rawImage == "" && inspect.Config != nil {
			rawImage = inspect.Config.Image
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
			ContainerID:  shortContainerID(c.ID),
			Stack:        stackName,
			Service:      svcName,
			StackService: stackService,
			Container:    cName,
			Image:        formatContainerImage(rawImage, fullImage),
			Status:       status,
			IP:           ipStr,
			Ports:        ports,
		})
	}

	// If -a / --all is requested, discover all services across workspace compose stacks
	if showAll {
		orderedStacks, composeMap, err := docker.DiscoverStacks(workDir)
		if err == nil {
			for _, stack := range orderedStacks {
				composePath := composeMap[stack]
				cfg, err := docker.ParseComposeFile(composePath)
				if err != nil {
					continue
				}

				for svcName, svc := range cfg.Services {
					stackServiceKey := fmt.Sprintf("%s/%s", stack, svcName)
					cName := svc.ContainerName
					if cName == "" {
						cName = svcName
					}

					// Check if container already listed
					if existingServices[stackServiceKey] || (svc.ContainerName != "" && existingServices[svc.ContainerName]) {
						continue
					}

					stackService := formatStackService(stack, svcName)

					// Target filtering check for missing services
					if targetMap != nil {
						matched := targetMap[cName] || targetMap[svcName] || targetMap[stack] || targetMap[stackService]
						if !matched {
							for _, arg := range args {
								argClean := strings.TrimPrefix(arg, "/")
								if strings.EqualFold(cName, argClean) ||
									strings.EqualFold(svcName, argClean) ||
									strings.EqualFold(stack, argClean) ||
									strings.EqualFold(strings.TrimPrefix(stackService, "/"), argClean) ||
									strings.Contains(strings.ToLower(cName), strings.ToLower(argClean)) {
									matched = true
									break
								}
							}
						}
						if !matched {
							continue
						}
					}

					resImage, resPorts := resolveServiceImageAndPorts(workDir, composePath, cfg, svc, 0)

					// Format ports from compose definition if available
					var portStrs []string
					for _, p := range resPorts {
						portStrs = append(portStrs, p)
					}
					portsDisplay := "-"
					if len(portStrs) > 0 {
						portsDisplay = strings.Join(portStrs, ", ")
					}

					items = append(items, containerStatusItem{
						ContainerID:  "-",
						Stack:        stack,
						Service:      svcName,
						StackService: stackService,
						Container:    cName,
						Image:        formatContainerImage(resImage, fullImage),
						Status:       "missing",
						IP:           "-",
						Ports:        portsDisplay,
					})
				}
			}
		}
	}

	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No containers found matching specified targets.")
		return nil
	}

	// Sort items by Stack, then Service
	sort.Slice(items, func(i, j int) bool {
		if items[i].Stack != items[j].Stack {
			return items[i].Stack < items[j].Stack
		}
		return items[i].Service < items[j].Service
	})

	renderStatusTable(cmd.OutOrStdout(), items)

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
