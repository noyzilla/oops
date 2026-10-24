package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

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

// ExtractContainerHostnames extracts all configured hostnames from compose service names, labels, and Config.Hostname
func ExtractContainerHostnames(inspect *container.InspectResponse) []string {
	if inspect == nil {
		return nil
	}

	tld := os.Getenv("OOPS_DNS_TLD")
	if tld == "" {
		tld = "oops"
	}

	var result []string
	seen := make(map[string]bool)

	add := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || trimmed == "localhost" {
			return
		}
		lower := strings.ToLower(trimmed)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, trimmed)
		}
	}

	// 1. Check explicit label override (supports comma and space separation)
	if inspect.Config != nil && inspect.Config.Labels != nil {
		if val, ok := inspect.Config.Labels["oops.dns.hostname"]; ok && val != "" {
			parts := strings.FieldsFunc(val, func(r rune) bool {
				return r == ',' || r == ' '
			})
			for _, p := range parts {
				add(p)
				if tld != "oops" && strings.HasSuffix(p, ".oops") {
					add(strings.TrimSuffix(p, ".oops") + "." + tld)
				}
			}
		}
	}

	// 2. Check container Config.Hostname
	if inspect.Config != nil && inspect.Config.Hostname != "" {
		h := inspect.Config.Hostname
		// Skip short docker default container IDs (usually 12 hex chars) if compose service name exists
		isDefaultHexID := len(h) == 12 && inspect.Config.Labels != nil && inspect.Config.Labels["com.docker.compose.service"] != ""
		if !isDefaultHexID {
			add(h)
			if tld != "oops" && strings.HasSuffix(h, ".oops") {
				add(strings.TrimSuffix(h, ".oops") + "." + tld)
			}
		}
	}

	// 3. Auto-register Compose Service Name: <service>.<tld> (and <service>.oops)
	if inspect.Config != nil && inspect.Config.Labels != nil {
		if service, ok := inspect.Config.Labels["com.docker.compose.service"]; ok && service != "" {
			add(fmt.Sprintf("%s.%s", service, tld))
			if tld != "oops" {
				add(fmt.Sprintf("%s.oops", service))
			}
		}
	}

	return result
}

// ExtractContainerDNSMappings extracts explicit domain -> IP mappings from oops.dns.mappings label
func ExtractContainerDNSMappings(inspect *container.InspectResponse) map[string]net.IP {
	if inspect == nil || inspect.Config == nil || inspect.Config.Labels == nil {
		return nil
	}
	raw, ok := inspect.Config.Labels["oops.dns.mappings"]
	if !ok || strings.TrimSpace(raw) == "" {
		return nil
	}
	result := make(map[string]net.IP)
	pairs := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' '
	})
	for _, p := range pairs {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			domain := strings.TrimSpace(strings.ToLower(kv[0]))
			ip := net.ParseIP(strings.TrimSpace(kv[1]))
			if domain != "" && ip != nil {
				result[domain] = ip
			}
		}
	}
	return result
}

// ExtractContainerHostname extracts primary hostname from Config or label
func ExtractContainerHostname(inspect *container.InspectResponse) string {
	names := ExtractContainerHostnames(inspect)
	if len(names) > 0 {
		return names[0]
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

		// 1. Explicit domain -> IP mappings
		customMappings := ExtractContainerDNSMappings(&inspect)
		for domain, ip := range customMappings {
			r.Register(c.ID, domain, ip)
			log.Printf("[DNS] Registered custom mapping %q -> %v (Container: %s)", domain, ip, inspect.Name)
		}

		// 2. Standard hostnames with primary container IP
		hostnames := ExtractContainerHostnames(&inspect)
		ip := ExtractContainerIP(&inspect)
		if len(hostnames) > 0 && ip != nil {
			for _, h := range hostnames {
				r.Register(c.ID, h, ip)
				log.Printf("[DNS] Registered initial host %q -> %v (Container: %s)", h, ip, inspect.Name)
			}
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
					customMappings := ExtractContainerDNSMappings(&inspect)
					for domain, ip := range customMappings {
						r.Register(event.Actor.ID, domain, ip)
						log.Printf("[DNS] Added custom mapping %q -> %v (Container: %s)", domain, ip, inspect.Name)
					}

					hostnames := ExtractContainerHostnames(&inspect)
					ip := ExtractContainerIP(&inspect)
					if len(hostnames) > 0 && ip != nil {
						for _, h := range hostnames {
							r.Register(event.Actor.ID, h, ip)
							log.Printf("[DNS] Added host %q -> %v (Container: %s)", h, ip, inspect.Name)
						}
					}
				}
			case "die", "stop", "pause", "destroy":
				r.Unregister(event.Actor.ID)
				log.Printf("[DNS] Removed hosts for container ID: %s", event.Actor.ID[:12])
			}
		}
	}
}
