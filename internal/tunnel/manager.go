package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// TunnelStatus represents current status of a running or stopped tunnel
type TunnelStatus struct {
	RemoteName string   `json:"remote_name"`
	Container  string   `json:"container"`
	Status     string   `json:"status"`
	State      string   `json:"state"`
	Hostname   string   `json:"hostname"`
	Endpoints  []string `json:"endpoints"`
	Services   string   `json:"services"`
}

// Manager orchestrates tunnel lifecycle on Docker
type Manager struct {
	dockerCli *client.Client
	runner    CommandRunner
}

// NewManager creates an initialized tunnel Manager
func NewManager(dockerCli *client.Client, runner CommandRunner) *Manager {
	if runner == nil {
		runner = DefaultCommandRunner
	}
	return &Manager{
		dockerCli: dockerCli,
		runner:    runner,
	}
}

// FindAvailableIPs finds count unused IP addresses in a CIDR subnet
func FindAvailableIPs(subnetCIDR string, usedIPs map[string]bool, count int) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("only IPv4 subnets supported: %s", subnetCIDR)
	}

	var available []string
	cur := make(net.IP, 4)
	copy(cur, ipv4.Mask(ipNet.Mask))

	inc := func(target net.IP) {
		for j := len(target) - 1; j >= 0; j-- {
			target[j]++
			if target[j] > 0 {
				break
			}
		}
	}

	for i := 0; i < 254; i++ {
		inc(cur)
		if !ipNet.Contains(cur) {
			break
		}
		last := cur[3]
		if last == 0 || last == 1 || last == 255 {
			continue
		}
		ipStr := cur.String()
		if !usedIPs[ipStr] {
			available = append(available, ipStr)
			usedIPs[ipStr] = true
			if len(available) == count {
				return available, nil
			}
		}
	}

	return available, nil
}

// EnsureNetwork finds or creates the primary Oops Docker network (oops_edge or bridge)
func (m *Manager) EnsureNetwork(ctx context.Context) (string, error) {
	networks, err := m.dockerCli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "bridge", nil
	}

	for _, n := range networks {
		if n.Name == "oops_edge" {
			return "oops_edge", nil
		}
	}
	for _, n := range networks {
		if n.Name == "oops_db" {
			return "oops_db", nil
		}
	}

	return "bridge", nil
}

// EnsureImage checks if image exists locally; pulls if missing
func (m *Manager) EnsureImage(ctx context.Context, imgName string) error {
	_, _, err := m.dockerCli.ImageInspectWithRaw(ctx, imgName)
	if err == nil {
		return nil
	}

	out, err := m.dockerCli.ImagePull(ctx, imgName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed pulling image %s: %w", imgName, err)
	}
	defer out.Close()
	_, _ = io.Copy(io.Discard, out)
	return nil
}

// Up starts or updates the tunnel container for the given configuration
func (m *Manager) Up(ctx context.Context, cfg *TunnelConfig, mappings []PortMapping) (*TunnelStatus, error) {
	// 1. Collect unique service names for remote IP resolution
	var serviceNames []string
	seenSvc := make(map[string]bool)
	for _, mp := range mappings {
		if !seenSvc[mp.Service] {
			seenSvc[mp.Service] = true
			serviceNames = append(serviceNames, mp.Service)
		}
	}

	// 2. Resolve remote container metadata (IP and Exposed Ports)
	resolvedMeta, err := ResolveRemoteServices(ctx, m.runner, cfg.Host, serviceNames)
	if err != nil {
		return nil, err
	}

	for i := range mappings {
		meta := resolvedMeta[mappings[i].Service]
		mappings[i].ContainerIP = meta.IP

		// Auto-discover port if not explicitly specified
		if mappings[i].RemotePort == 0 {
			if len(meta.ExposedPorts) > 0 {
				mappings[i].RemotePort = meta.ExposedPorts[0]
			} else {
				mappings[i].RemotePort = WellKnownDefaultPort(mappings[i].Service)
			}
			mappings[i].LocalPort = mappings[i].RemotePort
		}
	}

	// 3. Remove existing container if present
	cName := cfg.ContainerName()
	_ = m.Down(ctx, cfg.RemoteName)

	// 4. Ensure image and network
	if err := m.EnsureImage(ctx, TunnelImage); err != nil {
		return nil, err
	}

	targetNet, err := m.EnsureNetwork(ctx)
	if err != nil {
		targetNet = "bridge"
	}

	// 5. Inspect target network and allocate local IPs
	var primaryIP string
	portCounts := make(map[int]int)
	for _, mp := range mappings {
		portCounts[mp.LocalPort]++
	}
	hasPortCollision := false
	for _, cnt := range portCounts {
		if cnt > 1 {
			hasPortCollision = true
			break
		}
	}

	netInspect, netErr := m.dockerCli.NetworkInspect(ctx, targetNet, network.InspectOptions{})
	if netErr == nil && len(netInspect.IPAM.Config) > 0 {
		subnet := netInspect.IPAM.Config[0].Subnet
		usedIPs := make(map[string]bool)
		for _, endpoint := range netInspect.Containers {
			if endpoint.IPv4Address != "" {
				addr := strings.Split(endpoint.IPv4Address, "/")[0]
				usedIPs[addr] = true
			}
		}

		neededIPs := len(mappings)
		allocated, err := FindAvailableIPs(subnet, usedIPs, neededIPs)
		if err == nil && len(allocated) >= neededIPs {
			primaryIP = allocated[0]
			if hasPortCollision {
				for i := range mappings {
					mappings[i].LocalIP = allocated[i]
				}
			} else {
				for i := range mappings {
					mappings[i].LocalIP = primaryIP
				}
			}
		}
	}

	// 6. Build container config
	cConfig, hConfig, err := BuildContainerConfig(cfg, mappings)
	if err != nil {
		return nil, fmt.Errorf("failed constructing container config: %w", err)
	}

	var netConfig *network.NetworkingConfig
	if targetNet != "" && targetNet != "default" {
		endpoint := &network.EndpointSettings{}
		if primaryIP != "" {
			endpoint.IPAMConfig = &network.EndpointIPAMConfig{
				IPv4Address: primaryIP,
			}
		}
		netConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				targetNet: endpoint,
			},
		}
	}

	// 7. Create container
	resp, err := m.dockerCli.ContainerCreate(ctx, cConfig, hConfig, netConfig, nil, cName)
	if err != nil {
		return nil, fmt.Errorf("failed creating tunnel container %s: %w", cName, err)
	}

	// 8. Start container
	if err := m.dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed starting tunnel container %s: %w", cName, err)
	}

	// 9. Wait briefly to verify it didn't exit immediately
	time.Sleep(1 * time.Second)
	inspect, err := m.dockerCli.ContainerInspect(ctx, resp.ID)
	if err == nil && !inspect.State.Running {
		return nil, fmt.Errorf("tunnel container exited unexpectedly: %s (exit code %d)", inspect.State.Error, inspect.State.ExitCode)
	}

	var endpoints []string
	for _, m := range mappings {
		if cfg.Mode == "mesh" {
			endpoints = append(endpoints, fmt.Sprintf("%s:%d (mesh %s)", cfg.ServiceDomain(m.Service), m.RemotePort, cfg.Subnet))
		} else {
			endpoints = append(endpoints, fmt.Sprintf("%s:%d", cfg.ServiceDomain(m.Service), m.LocalPort))
		}
	}

	return &TunnelStatus{
		RemoteName: cfg.RemoteName,
		Container:  cName,
		Status:     "Up",
		State:      "running",
		Hostname:   cfg.Hostname,
		Endpoints:  endpoints,
		Services:   cConfig.Labels[LabelTunnelServices],
	}, nil
}

// Down stops and removes the tunnel container for a remote
func (m *Manager) Down(ctx context.Context, remoteName string) error {
	cleanRemote := strings.TrimPrefix(strings.TrimSpace(remoteName), "@")
	cName := "oops-tunnel-" + cleanRemote

	inspect, err := m.dockerCli.ContainerInspect(ctx, cName)
	if err != nil {
		return nil
	}

	stopTimeout := 5
	_ = m.dockerCli.ContainerStop(ctx, inspect.ID, container.StopOptions{Timeout: &stopTimeout})
	return m.dockerCli.ContainerRemove(ctx, inspect.ID, container.RemoveOptions{Force: true})
}

// List returns all active or stopped tunnel containers
func (m *Manager) List(ctx context.Context) ([]TunnelStatus, error) {
	f := filters.NewArgs()
	f.Add("label", LabelTunnelRemote)

	containers, err := m.dockerCli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("failed listing tunnel containers: %w", err)
	}

	var list []TunnelStatus
	for _, c := range containers {
		rName := c.Labels[LabelTunnelRemote]
		hostname := c.Labels[LabelTunnelHostname]
		if hostname == "" {
			hostname = fmt.Sprintf("tunnel-%s.oops", rName)
		}

		servicesStr := c.Labels[LabelTunnelServices]
		var endpoints []string
		if servicesStr != "" {
			parts := strings.Split(servicesStr, ",")
			for _, p := range parts {
				// Format: svc:local->ip:remote
				if idx := strings.Index(p, "->"); idx != -1 {
					head := p[:idx]
					sub := strings.Split(head, ":")
					if len(sub) == 2 {
						endpoints = append(endpoints, fmt.Sprintf("%s.%s:%s", sub[0], rName, sub[1]))
					}
				}
			}
		}

		cName := ""
		if len(c.Names) > 0 {
			cName = strings.TrimPrefix(c.Names[0], "/")
		}

		list = append(list, TunnelStatus{
			RemoteName: rName,
			Container:  cName,
			Status:     c.Status,
			State:      c.State,
			Hostname:   hostname,
			Endpoints:  endpoints,
			Services:   servicesStr,
		})
	}

	return list, nil
}
