package tunnel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/remote"
)

// DefaultServices defines default services to tunnel when none are specified
var DefaultServices = []string{"mysql", "redis"}

// PortMapping represents an individual forwarded port
type PortMapping struct {
	Service     string `json:"service"`
	LocalPort   int    `json:"local_port"`
	RemotePort  int    `json:"remote_port"`
	LocalIP     string `json:"local_ip,omitempty"`
	ContainerIP string `json:"container_ip,omitempty"`
}

// String returns formatted mapping representation
func (m PortMapping) String() string {
	if m.ContainerIP != "" {
		return fmt.Sprintf("%s (%s:%d -> :%d)", m.Service, m.ContainerIP, m.RemotePort, m.LocalPort)
	}
	return fmt.Sprintf("%s (:%d -> :%d)", m.Service, m.RemotePort, m.LocalPort)
}

// TunnelConfig holds the resolved parameters for a tunnel connection
type TunnelConfig struct {
	RemoteName        string   `json:"remote_name"`
	Host              string   `json:"host"`
	Path              string   `json:"path,omitempty"`
	Hostname          string   `json:"hostname"`
	Services          []string `json:"services,omitempty"`
	Mode              string   `json:"mode,omitempty"`   // "port" or "mesh"
	Subnet            string   `json:"subnet,omitempty"` // CIDR (e.g. 10.240.0.0/16)
	MemoryLimit       int64    `json:"memory_limit,omitempty"`
	MemoryReservation int64    `json:"memory_reservation,omitempty"`
}

// ServiceDomain returns Single-TLD remote domain: <service>.<remote>
func (c *TunnelConfig) ServiceDomain(service string) string {
	return fmt.Sprintf("%s.%s", service, c.RemoteName)
}

// LegacyServiceDomain returns legacy subdomain: <service>.<remote>.tunnel.oops
func (c *TunnelConfig) LegacyServiceDomain(service string) string {
	return fmt.Sprintf("%s.%s.tunnel.oops", service, c.RemoteName)
}

// ContainerName returns standard container name for the tunnel
func (c *TunnelConfig) ContainerName() string {
	return "oops-tunnel-" + c.RemoteName
}

// ParsePortSpecs parses service, service:port or service:local:remote mappings.
// If specs is empty, it uses fallbackServices (from remote.yaml or DefaultServices).
func ParsePortSpecs(specs []string, fallbackServices []string) ([]PortMapping, error) {
	if len(specs) == 0 {
		if len(fallbackServices) > 0 {
			specs = fallbackServices
		} else {
			specs = DefaultServices
		}
	}

	var mappings []PortMapping
	seenServices := make(map[string]bool)

	for _, spec := range specs {
		clean := strings.TrimSpace(spec)
		if clean == "" {
			continue
		}

		parts := strings.Split(clean, ":")
		if len(parts) > 3 {
			return nil, fmt.Errorf("invalid port spec %q: expected <service>, <service>:<port>, or <service>:<local>:<remote>", spec)
		}

		service := strings.TrimSpace(parts[0])
		if service == "" {
			return nil, fmt.Errorf("invalid port spec %q: service name cannot be empty", spec)
		}

		if seenServices[service] {
			return nil, fmt.Errorf("duplicate service %q specified in port specs", service)
		}
		seenServices[service] = true

		var localPort, remotePort int

		if len(parts) == 1 {
			// Auto-discover port from remote container metadata
			localPort = 0
			remotePort = 0
		} else if len(parts) == 2 {
			port, err := strconv.Atoi(parts[1])
			if err != nil || port <= 0 || port > 65535 {
				return nil, fmt.Errorf("invalid port in spec %q: %v", spec, parts[1])
			}
			localPort = port
			remotePort = port
		} else {
			var err error
			localPort, err = strconv.Atoi(parts[1])
			if err != nil || localPort <= 0 || localPort > 65535 {
				return nil, fmt.Errorf("invalid local port in spec %q: %v", spec, parts[1])
			}
			remotePort, err = strconv.Atoi(parts[2])
			if err != nil || remotePort <= 0 || remotePort > 65535 {
				return nil, fmt.Errorf("invalid remote port in spec %q: %v", spec, parts[2])
			}
		}

		mappings = append(mappings, PortMapping{
			Service:    service,
			LocalPort:  localPort,
			RemotePort: remotePort,
		})
	}

	return mappings, nil
}

// ResolveTunnelConfig reads /@<remote>/remote.yaml and builds TunnelConfig
func ResolveTunnelConfig(workDir, remoteTarget string) (*TunnelConfig, error) {
	cleanRemote := strings.TrimPrefix(strings.TrimSpace(remoteTarget), "@")
	if cleanRemote == "" {
		return nil, fmt.Errorf("remote target cannot be empty")
	}

	cfg, err := remote.LoadConfig(workDir, cleanRemote)
	if err != nil {
		return nil, fmt.Errorf("failed to load remote config for @%s: %w", cleanRemote, err)
	}

	host := cfg.Host
	if host == "" {
		host = cleanRemote
	}

	hostname := fmt.Sprintf("tunnel-%s.oops", cleanRemote)

	mode := strings.ToLower(strings.TrimSpace(cfg.Tunnel.Mode))
	if mode == "" {
		mode = "port"
	}

	subnet := ""
	if capsuleOops, err := docker.LoadOopsConfig(remote.CapsuleDir(workDir, cleanRemote)); err == nil && capsuleOops != nil && capsuleOops.Network.Subnet != "" {
		subnet = capsuleOops.Network.Subnet
	} else if cfg.Network.Subnet != "" {
		subnet = cfg.Network.Subnet
	} else {
		subnet = "10.240.0.0/16"
	}

	var memLimit, memReservation int64
	if memStr := strings.TrimSpace(cfg.Tunnel.Memory); memStr != "" {
		if bytes, err := units.RAMInBytes(memStr); err == nil && bytes > 0 {
			memLimit = bytes
			memReservation = bytes / 2
		}
	}

	return &TunnelConfig{
		RemoteName:        cleanRemote,
		Host:              host,
		Path:              cfg.Path,
		Hostname:          hostname,
		Services:          cfg.Tunnel.Services,
		Mode:              mode,
		Subnet:            subnet,
		MemoryLimit:       memLimit,
		MemoryReservation: memReservation,
	}, nil
}
