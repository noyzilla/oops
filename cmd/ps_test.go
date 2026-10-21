package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func TestFormatContainerPorts(t *testing.T) {
	tests := []struct {
		name     string
		inspect  *container.InspectResponse
		expected string
	}{
		{
			name:     "Nil or empty network settings",
			inspect:  &container.InspectResponse{},
			expected: "-",
		},
		{
			name: "All-interface TCP deduplicated and internal port without parentheses",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "80"},
								{HostIP: "::", HostPort: "80"},
							},
							"443/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "443"},
								{HostIP: "::", HostPort: "443"},
							},
							"2019/tcp": nil,
						},
					},
				},
			},
			expected: "*:80, *:443, 2019",
		},
		{
			name: "All-interface UDP port formatting without redundant container port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"53/udp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "53"},
							},
						},
					},
				},
			},
			expected: "*:53/udp",
		},
		{
			name: "Multiple host ports mapped to same container port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"9000/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "9001"},
								{HostIP: "0.0.0.0", HostPort: "9002"},
								{HostIP: "0.0.0.0", HostPort: "9003"},
							},
						},
					},
				},
			},
			expected: "*:[9001,9002,9003]->9000",
		},
		{
			name: "Loopback 127.0.0.1 Host IP binding with #: prefix and omitted same port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8080/tcp": []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "8080"},
							},
						},
					},
				},
			},
			expected: "#:8080",
		},
		{
			name: "Loopback 127.0.0.1 Host IP binding with different ports",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8000/tcp": []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "8080"},
							},
						},
					},
				},
			},
			expected: "#:8080->8000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatContainerPorts(tt.inspect)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatContainerImage(t *testing.T) {
	raw := "asia-southeast1-docker.pkg.dev/my-project/my-repo/web-api:v1.2.3"
	if got := formatContainerImage(raw, false); got != "web-api:v1.2.3" {
		t.Errorf("formatContainerImage(raw, false) = %q, want %q", got, "web-api:v1.2.3")
	}
	if got := formatContainerImage(raw, true); got != raw {
		t.Errorf("formatContainerImage(raw, true) = %q, want %q", got, raw)
	}
	if got := formatContainerImage("mysql:8.4", false); got != "mysql:8.4" {
		t.Errorf("formatContainerImage(mysql:8.4, false) = %q, want mysql:8.4", got)
	}
}

func TestExtractContainerStackName(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project":              "oopsbox",
		"com.docker.compose.project.config_files": "/home/captain/oopsbox/compose.yaml,/home/captain/oopsbox/compose.edge.yaml",
	}
	if got := extractContainerStackName(labels); got != "edge" {
		t.Errorf("extractContainerStackName() = %q, want %q", got, "edge")
	}
	if got := formatStackService("edge", "caddy-proxy"); got != "/edge/caddy-proxy" {
		t.Errorf("formatStackService() = %q, want /edge/caddy-proxy", got)
	}
}

func TestNormalizeCLIArgs(t *testing.T) {
	got := NormalizeCLIArgs([]string{"ps", "-fl"})
	if len(got) != 2 || got[0] != "ps" || got[1] != "--full" {
		t.Fatalf("NormalizeCLIArgs([ps, -fl]) = %v, want [ps, --full]", got)
	}
}

func TestRenderStatusTableAlignment(t *testing.T) {
	items := []containerStatusItem{
		{
			ContainerID:  "3d285d623b64",
			Stack:        "edge",
			Service:      "caddy-proxy",
			StackService: "/edge/caddy-proxy",
			Image:        "caddy-docker-proxy:alpine",
			Status:       "Up About a minute",
			IP:           "172.18.0.2",
			Ports:        "*:80, *:443, 2019",
		},
		{
			ContainerID:  "64cd605d59a8",
			Stack:        "db",
			Service:      "mysql",
			StackService: "/db/mysql",
			Image:        "mysql:8.4",
			Status:       "Up About a minute (healthy)",
			IP:           "172.20.0.3",
			Ports:        "#:3306, 33060",
		},
	}

	var buf bytes.Buffer
	renderStatusTable(&buf, items)
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (1 header + 2 rows), got %d:\n%s", len(lines), buf.String())
	}

	header := lines[0]
	row1 := lines[1]

	checks := []struct {
		headerCol string
		rowVal    string
	}{
		{"CONTAINER ID", "3d285d623b64"},
		{"STACK", "edge"},
		{"SERVICE", "caddy-proxy"},
		{"IMAGE", "caddy-docker-proxy:alpine"},
		{"STATUS", "Up About a minute"},
		{"IP", "172.18.0.2"},
		{"PORTS (*=all, #=local)", "*:80, *:443, 2019"},
	}

	for _, c := range checks {
		hIdx := strings.Index(header, c.headerCol)
		rIdx := strings.Index(row1, c.rowVal)
		if hIdx == -1 || rIdx == -1 || hIdx != rIdx {
			t.Errorf("column %q index (%d) != row value %q index (%d)\nHeader: %s\nRow 1 : %s",
				c.headerCol, hIdx, c.rowVal, rIdx, header, row1)
		}
	}
}

func TestRenderStatusTableMissingService(t *testing.T) {
	items := []containerStatusItem{
		{
			ContainerID:  "-",
			Stack:        "db",
			Service:      "redis",
			StackService: "/db/redis",
			Image:        "redis:alpine",
			Status:       "missing",
			IP:           "-",
			Ports:        "#:6379",
		},
	}

	var buf bytes.Buffer
	renderStatusTable(&buf, items)
	output := buf.String()
	if !strings.Contains(output, "missing") {
		t.Errorf("expected table output to contain 'missing', got:\n%s", output)
	}
	if !strings.Contains(output, "redis") {
		t.Errorf("expected table output to contain 'redis', got:\n%s", output)
	}
	if !strings.Contains(output, "STACK") || !strings.Contains(output, "SERVICE") {
		t.Errorf("expected header to contain separate STACK and SERVICE columns, got:\n%s", output)
	}
}

