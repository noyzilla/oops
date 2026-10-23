package docker

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// GetContainerIP inspects a container by name and returns its IP address.
// If preferNetwork is specified (e.g. "oops_edge"), it attempts to retrieve the IP
// on that specific network first before falling back to any available network IP.
func GetContainerIP(containerName string, preferNetwork string) string {
	format := "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{break}}{{end}}"
	if preferNetwork != "" {
		format = fmt.Sprintf("{{with index .NetworkSettings.Networks %q}}{{.IPAddress}}{{else}}{{range .NetworkSettings.Networks}}{{if .IPAddress}}{{.IPAddress}}{{break}}{{end}}{{end}}{{end}}", preferNetwork)
	}
	cmd := exec.Command("docker", "inspect", "-f", format, containerName)
	out, err := cmd.Output()
	if err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" && ip != "<no value>" {
			return ip
		}
	}
	return ""
}

// GetOopsContainerIP returns the network IP address of the running 'oops' container,
// preferring its IP on the 'oops_edge' network.
// If running directly inside the container and docker inspect is unavailable, it falls back
// to resolving the container's hostname.
func GetOopsContainerIP() string {
	ip := GetContainerIP("oops", "oops_edge")
	if ip != "" {
		return ip
	}

	// Fallback if running inside the oops container
	if hostname, err := os.Hostname(); err == nil {
		if addrs, err := net.LookupHost(hostname); err == nil && len(addrs) > 0 {
			for _, a := range addrs {
				if !strings.HasPrefix(a, "127.") && !strings.Contains(a, ":") {
					return a
				}
			}
		}
	}

	return ""
}

