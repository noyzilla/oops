package dns

import (
	"context"
	"log"
	"net"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// ExtractContainerIP finds the first non-loopback IPv4 address assigned to the container
func ExtractContainerIP(inspect *container.InspectResponse) net.IP {
	if inspect == nil || inspect.NetworkSettings == nil {
		return nil
	}

	// 1. Check IPAddress at root
	if inspect.NetworkSettings.IPAddress != "" {
		if ip := net.ParseIP(inspect.NetworkSettings.IPAddress); ip != nil {
			return ip
		}
	}

	// 2. Check all attached networks
	for _, endpoint := range inspect.NetworkSettings.Networks {
		if endpoint != nil && endpoint.IPAddress != "" {
			if ip := net.ParseIP(endpoint.IPAddress); ip != nil {
				return ip
			}
		}
	}

	return nil
}

// ExtractContainerHostname extracts hostname from Config or label
func ExtractContainerHostname(inspect *container.InspectResponse) string {
	if inspect == nil {
		return ""
	}

	// Check explicit label override first
	if inspect.Config != nil && inspect.Config.Labels != nil {
		if val, ok := inspect.Config.Labels["oops.dns.hostname"]; ok && val != "" {
			return val
		}
	}

	// Check container Config.Hostname
	if inspect.Config != nil && inspect.Config.Hostname != "" {
		return inspect.Config.Hostname
	}

	return ""
}

// SyncInitialContainers scans all running containers and registers their hostnames
func SyncInitialContainers(ctx context.Context, cli *client.Client, r *Resolver) error {
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containers {
		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		hostname := ExtractContainerHostname(&inspect)
		ip := ExtractContainerIP(&inspect)
		if hostname != "" && ip != nil {
			r.Register(c.ID, hostname, ip)
			log.Printf("[DNS] Registered initial host %q -> %v (Container: %s)", hostname, ip, inspect.Name)
		}
	}

	return nil
}

// WatchDockerEvents listens to real-time container lifecycle events and synchronizes the resolver table
func WatchDockerEvents(ctx context.Context, cli *client.Client, r *Resolver) {
	if err := SyncInitialContainers(ctx, cli, r); err != nil {
		log.Printf("[DNS] Warning: Initial container scan failed: %v", err)
	}

	f := filters.NewArgs()
	f.Add("type", "container")

	msgChan, errChan := cli.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			if err != nil {
				log.Printf("[DNS] Event watcher error: %v", err)
				return
			}
		case event := <-msgChan:
			switch event.Action {
			case "start", "unpause":
				inspect, err := cli.ContainerInspect(ctx, event.Actor.ID)
				if err == nil {
					hostname := ExtractContainerHostname(&inspect)
					ip := ExtractContainerIP(&inspect)
					if hostname != "" && ip != nil {
						r.Register(event.Actor.ID, hostname, ip)
						log.Printf("[DNS] Added host %q -> %v (Container: %s)", hostname, ip, inspect.Name)
					}
				}
			case "die", "stop", "pause", "destroy":
				r.Unregister(event.Actor.ID)
				log.Printf("[DNS] Removed hosts for container ID: %s", event.Actor.ID[:12])
			}
		}
	}
}
