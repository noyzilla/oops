package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/noyzilla/oops/internal/docker"
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
	// Sub-stack compose file
	labelsEdge := map[string]string{
		"com.docker.compose.project":              "oopsbox",
		"com.docker.compose.project.config_files": "/home/captain/oopsbox/compose.yaml,/home/captain/oopsbox/compose.edge.yaml",
	}
	if got := extractContainerStackName(labelsEdge); got != "/edge" {
		t.Errorf("extractContainerStackName(edge) = %q, want %q", got, "/edge")
	}

	// Root compose file
	labelsRoot := map[string]string{
		"com.docker.compose.project":              "oopsbox",
		"com.docker.compose.project.config_files": "/home/captain/oopsbox/compose.yaml",
	}
	if got := extractContainerStackName(labelsRoot); got != "/" {
		t.Errorf("extractContainerStackName(root) = %q, want %q", got, "/")
	}

	// Tunnel container
	labelsTunnel := map[string]string{
		"oops.tunnel.remote": "b9tpe",
	}
	if got := extractContainerStackName(labelsTunnel, "oops-tunnel-b9tpe"); got != "/tunnel" {
		t.Errorf("extractContainerStackName(tunnel) = %q, want %q", got, "/tunnel")
	}

	// External container
	labelsExternal := map[string]string{
		"com.docker.compose.project": "third-party",
	}
	if got := extractContainerStackName(labelsExternal, "third-party-app"); got != "%" {
		t.Errorf("extractContainerStackName(external) = %q, want %q", got, "%")
	}

	// FormatStackService checks
	if got := formatStackService("/", "filebrowser"); got != "/filebrowser" {
		t.Errorf("formatStackService(/, filebrowser) = %q, want /filebrowser", got)
	}
	if got := formatStackService("/edge", "caddy-proxy"); got != "/edge/caddy-proxy" {
		t.Errorf("formatStackService(/edge, caddy-proxy) = %q, want /edge/caddy-proxy", got)
	}
	if got := formatStackService("/tunnel", "b9tpe"); got != "/tunnel/b9tpe" {
		t.Errorf("formatStackService(/tunnel, b9tpe) = %q, want /tunnel/b9tpe", got)
	}
	if got := formatStackService("%", "myapp"); got != "%myapp" {
		t.Errorf("formatStackService(%%, myapp) = %q, want %%myapp", got)
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
			Stack:        "/edge",
			Service:      "caddy-proxy",
			StackService: "/edge/caddy-proxy",
			Image:        "caddy-docker-proxy:alpine",
			Status:       "Up About a minute",
			IP:           "172.18.0.2",
			Ports:        "*:80, *:443, 2019",
		},
		{
			ContainerID:  "64cd605d59a8",
			Stack:        "/db",
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
		{"STACK", "/edge"},
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
			Stack:        "/db",
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

func TestResolveServiceImageAndPorts_Extends(t *testing.T) {
	tmpDir := t.TempDir()

	commonYAML := `services:
  base-app:
    image: myrepo/base-app:1.2.3
    ports:
      - "8080:8080"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "common.yaml"), []byte(commonYAML), 0644); err != nil {
		t.Fatal(err)
	}

	appYAML := `services:
  web:
    extends:
      file: common.yaml
      service: base-app
    container_name: web-app
`
	appPath := filepath.Join(tmpDir, "compose.app.yaml")
	if err := os.WriteFile(appPath, []byte(appYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := docker.ParseComposeFile(appPath)
	if err != nil {
		t.Fatalf("failed parsing compose file: %v", err)
	}

	svc := cfg.Services["web"]
	gotImg, gotPorts := resolveServiceImageAndPorts(tmpDir, appPath, cfg, svc, 0)
	if gotImg != "myrepo/base-app:1.2.3" {
		t.Errorf("resolveServiceImageAndPorts() image = %q, want %q", gotImg, "myrepo/base-app:1.2.3")
	}
	if len(gotPorts) != 1 || gotPorts[0] != "8080:8080" {
		t.Errorf("resolveServiceImageAndPorts() ports = %v, want ['8080:8080']", gotPorts)
	}
}

func TestStatusItemSorting(t *testing.T) {
	items := []containerStatusItem{
		{Stack: "%", Service: "my-external-app"},
		{Stack: "/edge", Service: "caddy-proxy"},
		{Stack: "/tunnel", Service: "b9tpe"},
		{Stack: "/db", Service: "redis"},
		{Stack: "/", Service: "homeassistant"},
		{Stack: "/db", Service: "mysql"},
		{Stack: "/", Service: "filebrowser"},
	}

	sort.Slice(items, func(i, j int) bool {
		stackWeight := func(s string) int {
			if s == "/" {
				return 0
			}
			if strings.HasPrefix(s, "/") {
				return 1
			}
			if s == "%" {
				return 3
			}
			return 2
		}
		wI := stackWeight(items[i].Stack)
		wJ := stackWeight(items[j].Stack)
		if wI != wJ {
			return wI < wJ
		}
		if items[i].Stack != items[j].Stack {
			return items[i].Stack < items[j].Stack
		}
		return items[i].Service < items[j].Service
	})

	expected := []struct {
		stack   string
		service string
	}{
		{"/", "filebrowser"},
		{"/", "homeassistant"},
		{"/db", "mysql"},
		{"/db", "redis"},
		{"/edge", "caddy-proxy"},
		{"/tunnel", "b9tpe"},
		{"%", "my-external-app"},
	}

	for i, exp := range expected {
		if items[i].Stack != exp.stack || items[i].Service != exp.service {
			t.Errorf("item[%d] = {%q, %q}, want {%q, %q}", i, items[i].Stack, items[i].Service, exp.stack, exp.service)
		}
	}
}

func TestCompactContainerStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Up 2 hours (healthy)", "Up 2h (healthy)"},
		{"Up 3 days (healthy)", "Up 3d (healthy)"},
		{"Up 1 hour", "Up 1h"},
		{"Up 45 minutes", "Up 45m"},
		{"Up 15 seconds", "Up 15s"},
		{"Up 1 second", "Up 1s"},
		{"Exited (0) 5 minutes ago", "Exited (0) 5m ago"},
		{"Restarting (1) 20 seconds ago", "Restarting (1) 20s ago"},
		{"Up About an hour", "Up ~1h"},
		{"Up Less than a second", "Up <1s"},
		{"Up About a minute", "Up ~1m"},
		{"Up 2 weeks", "Up 2w"},
		{"Up 3 months", "Up 3mo"},
		{"Up 1 year", "Up 1y"},
		{"missing", "missing"},
		{"-", "-"},
	}

	for _, tt := range tests {
		actual := compactContainerStatus(tt.input)
		if actual != tt.expected {
			t.Errorf("compactContainerStatus(%q) = %q, want %q", tt.input, actual, tt.expected)
		}
	}
}


