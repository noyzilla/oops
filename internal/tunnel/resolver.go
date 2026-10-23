package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CommandRunner defines the function signature for executing commands against remote instances
type CommandRunner func(ctx context.Context, host, remoteCmd string) (string, error)

// DefaultCommandRunner executes a command via standard ssh
var DefaultCommandRunner CommandRunner = runSSHCommand

func runSSHCommand(ctx context.Context, host, remoteCmd string) (string, error) {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		host,
		remoteCmd,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("SSH command to %s failed: %w (stderr: %s)", host, err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// RemoteServiceMetadata contains resolved IP and exposed ports for a service
type RemoteServiceMetadata struct {
	IP           string
	ExposedPorts []int
}

// BuildResolutionScript generates the remote shell script to query container bridge IPs and exposed ports
func BuildResolutionScript(services []string) string {
	var sb strings.Builder
	sb.WriteString("for svc in ")
	for _, s := range services {
		sb.WriteString(strings.TrimSpace(s) + " ")
	}
	sb.WriteString("; do ")
	sb.WriteString(`cid=$(docker ps --filter "label=com.docker.compose.service=$svc" -q | head -n1); `)
	sb.WriteString(`if [ -z "$cid" ]; then cid=$(docker ps --filter "name=$svc" -q | head -n1); fi; `)
	sb.WriteString(`if [ -n "$cid" ]; then `)
	sb.WriteString(`ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$cid"); `)
	sb.WriteString(`ports=$(docker inspect -f '{{range $p, $conf := .Config.ExposedPorts}}{{$p}} {{end}}' "$cid"); `)
	sb.WriteString(`echo "$svc|IP=$ip|PORTS=$ports"; `)
	sb.WriteString(`else echo "$svc|NOT_FOUND"; fi; `)
	sb.WriteString("done")
	return sb.String()
}

// WellKnownDefaultPort returns standard default port for common services
func WellKnownDefaultPort(service string) int {
	s := strings.ToLower(service)
	switch {
	case strings.Contains(s, "mysql"), strings.Contains(s, "mariadb"):
		return 3306
	case strings.Contains(s, "redis"), strings.Contains(s, "valkey"):
		return 6379
	case strings.Contains(s, "postgres"), strings.Contains(s, "pg"):
		return 5432
	case strings.Contains(s, "mongo"):
		return 27017
	case strings.Contains(s, "minio"):
		return 9000
	case strings.Contains(s, "elastic"):
		return 9200
	case strings.Contains(s, "rabbit"):
		return 5672
	default:
		return 8080
	}
}

// ParseResolutionOutput parses the remote bash output into RemoteServiceMetadata map
func ParseResolutionOutput(output string) (map[string]RemoteServiceMetadata, error) {
	result := make(map[string]RemoteServiceMetadata)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		svc := strings.TrimSpace(parts[0])
		if parts[1] == "NOT_FOUND" {
			continue
		}

		meta := RemoteServiceMetadata{}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			if k == "IP" {
				meta.IP = v
			} else if k == "PORTS" {
				rawPorts := strings.Fields(v)
				for _, rp := range rawPorts {
					cleanPort := strings.Split(rp, "/")[0]
					if portNum, err := strconv.Atoi(cleanPort); err == nil && portNum > 0 {
						meta.ExposedPorts = append(meta.ExposedPorts, portNum)
					}
				}
			}
		}

		if meta.IP != "" {
			result[svc] = meta
		}
	}

	return result, nil
}

// ResolveRemoteServices resolves container bridge IPs and ports for all services on the remote instance
func ResolveRemoteServices(ctx context.Context, runner CommandRunner, host string, services []string) (map[string]RemoteServiceMetadata, error) {
	if runner == nil {
		runner = DefaultCommandRunner
	}

	script := BuildResolutionScript(services)
	output, err := runner(ctx, host, script)
	if err != nil {
		return nil, fmt.Errorf("failed resolving remote container metadata on %s: %w", host, err)
	}

	result, err := ParseResolutionOutput(output)
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, s := range services {
		meta, ok := result[s]
		if !ok || meta.IP == "" {
			missing = append(missing, s)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("remote services not found or not running on @%s: %s", host, strings.Join(missing, ", "))
	}

	return result, nil
}
