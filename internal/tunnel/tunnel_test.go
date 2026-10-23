package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParsePortSpecs_Defaults(t *testing.T) {
	mappings, err := ParsePortSpecs(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []PortMapping{
		{Service: "mysql", LocalPort: 0, RemotePort: 0},
		{Service: "redis", LocalPort: 0, RemotePort: 0},
	}

	if !reflect.DeepEqual(mappings, expected) {
		t.Errorf("got %+v, want %+v", mappings, expected)
	}
}

func TestParsePortSpecs_FallbackServicesFromConfig(t *testing.T) {
	fallback := []string{"postgres:5432", "redis"}
	mappings, err := ParsePortSpecs(nil, fallback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings[0].Service != "postgres" || mappings[0].LocalPort != 5432 {
		t.Errorf("unexpected mapping for postgres: %+v", mappings[0])
	}
	if mappings[1].Service != "redis" || mappings[1].LocalPort != 0 {
		t.Errorf("unexpected mapping for redis: %+v", mappings[1])
	}
}

func TestParsePortSpecs_CustomAndPortCollisionAllowed(t *testing.T) {
	specs := []string{"mysql1:3306", "mysql2:3306", "redis"}
	mappings, err := ParsePortSpecs(specs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(mappings))
	}

	if mappings[0].LocalPort != 3306 || mappings[1].LocalPort != 3306 {
		t.Errorf("expected both to have port 3306, got %d and %d", mappings[0].LocalPort, mappings[1].LocalPort)
	}
	if mappings[2].LocalPort != 0 {
		t.Errorf("expected auto-discover port 0 for redis, got %d", mappings[2].LocalPort)
	}
}

func TestParsePortSpecs_Errors(t *testing.T) {
	invalidSpecs := [][]string{
		{"mysql:not-a-port"},
		{":3306"},
		{"mysql:0"},
		{"mysql:70000"},
		{"mysql:3306", "mysql:3307"}, // Duplicate service name
	}

	for _, specs := range invalidSpecs {
		_, err := ParsePortSpecs(specs, nil)
		if err == nil {
			t.Errorf("expected error for specs %v, got nil", specs)
		}
	}
}

func TestFindAvailableIPs(t *testing.T) {
	subnet := "172.18.0.0/16"
	used := map[string]bool{
		"172.18.0.2": true,
		"172.18.0.3": true,
	}

	ips, err := FindAvailableIPs(subnet, used, 2)
	if err != nil {
		t.Fatalf("FindAvailableIPs failed: %v", err)
	}

	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d", len(ips))
	}
	if ips[0] != "172.18.0.4" || ips[1] != "172.18.0.5" {
		t.Errorf("expected 172.18.0.4 and 172.18.0.5, got %v", ips)
	}
}

func TestResolveTunnelConfig_WithServices(t *testing.T) {
	tempDir := t.TempDir()
	capsuleDir := filepath.Join(tempDir, "@b9tpe")
	_ = os.MkdirAll(capsuleDir, 0755)

	yamlContent := `host: b9tpe
path: ~/oopsbox
tunnel:
  services:
    - mysql
    - redis
    - minio:9000
`
	_ = os.WriteFile(filepath.Join(capsuleDir, "remote.yaml"), []byte(yamlContent), 0644)

	cfg, err := ResolveTunnelConfig(tempDir, "@b9tpe")
	if err != nil {
		t.Fatalf("ResolveTunnelConfig failed: %v", err)
	}

	if cfg.RemoteName != "b9tpe" {
		t.Errorf("expected RemoteName b9tpe, got %s", cfg.RemoteName)
	}
	if len(cfg.Services) != 3 || cfg.Services[0] != "mysql" || cfg.Services[2] != "minio:9000" {
		t.Errorf("unexpected Services from remote.yaml: %v", cfg.Services)
	}
	if cfg.ServiceDomain("mysql") != "mysql.b9tpe.tunnel.oops" {
		t.Errorf("expected ServiceDomain mysql.b9tpe.tunnel.oops, got %s", cfg.ServiceDomain("mysql"))
	}
}

func TestResolveRemoteServices_MockSuccess(t *testing.T) {
	mockRunner := func(ctx context.Context, host, remoteCmd string) (string, error) {
		return "mysql|IP=172.18.0.5|PORTS=3306/tcp 33060/tcp \nredis|IP=172.18.0.6|PORTS=6379/tcp \n", nil
	}

	services := []string{"mysql", "redis"}
	meta, err := ResolveRemoteServices(context.Background(), mockRunner, "b9tpe", services)
	if err != nil {
		t.Fatalf("ResolveRemoteServices failed: %v", err)
	}

	if meta["mysql"].IP != "172.18.0.5" {
		t.Errorf("expected mysql IP 172.18.0.5, got %s", meta["mysql"].IP)
	}
	if len(meta["mysql"].ExposedPorts) != 2 || meta["mysql"].ExposedPorts[0] != 3306 {
		t.Errorf("expected mysql exposed port 3306, got %v", meta["mysql"].ExposedPorts)
	}
	if meta["redis"].IP != "172.18.0.6" || len(meta["redis"].ExposedPorts) != 1 || meta["redis"].ExposedPorts[0] != 6379 {
		t.Errorf("expected redis IP 172.18.0.6 and port 6379, got %+v", meta["redis"])
	}
}

func TestBuildTunnelCommand_StandardSSH(t *testing.T) {
	cfg := &TunnelConfig{
		RemoteName: "b9tpe",
		Host:       "b9tpe",
		Hostname:   "tunnel-b9tpe.oops",
	}

	mappings := []PortMapping{
		{Service: "mysql1", LocalPort: 3306, RemotePort: 3306, LocalIP: "172.18.0.10", ContainerIP: "172.18.0.5"},
		{Service: "mysql2", LocalPort: 3306, RemotePort: 3306, LocalIP: "172.18.0.11", ContainerIP: "172.18.0.6"},
	}

	cmd := BuildTunnelCommand(cfg, mappings)
	script := cmd[2]

	if !strings.Contains(script, "ip addr add 172.18.0.10/16 dev eth0") {
		t.Errorf("script missing alias command for 172.18.0.10: %s", script)
	}
	if !strings.Contains(script, "-L 172.18.0.10:3306:172.18.0.5:3306") {
		t.Errorf("script missing forward for mysql1: %s", script)
	}
	if !strings.Contains(script, "exec ssh ") {
		t.Errorf("script should execute standard ssh: %s", script)
	}
	if !strings.Contains(script, "IgnoreUnknown=UseKeychain") {
		t.Errorf("script missing IgnoreUnknown=UseKeychain: %s", script)
	}
	if !strings.Contains(script, "sed -i -E") {
		t.Errorf("script missing sed command to sanitize ssh config: %s", script)
	}
	if strings.Contains(script, "gcloud compute ssh") {
		t.Errorf("script should NOT use gcloud compute ssh: %s", script)
	}
}

func TestBuildContainerConfig_CapNetAdminAndDnsMappings(t *testing.T) {
	cfg := &TunnelConfig{
		RemoteName: "b9tpe",
		Host:       "b9tpe",
		Hostname:   "tunnel-b9tpe.oops",
	}

	mappings := []PortMapping{
		{Service: "mysql1", LocalPort: 3306, RemotePort: 3306, LocalIP: "172.18.0.10", ContainerIP: "172.18.0.5"},
		{Service: "mysql2", LocalPort: 3306, RemotePort: 3306, LocalIP: "172.18.0.11", ContainerIP: "172.18.0.6"},
	}

	cConfig, hConfig, err := BuildContainerConfig(cfg, mappings)
	if err != nil {
		t.Fatalf("BuildContainerConfig failed: %v", err)
	}

	if !strings.Contains(cConfig.Labels["oops.dns.hostname"], ".b9tpe.tunnel.oops") {
		t.Errorf("expected wildcard domain label, got %s", cConfig.Labels["oops.dns.hostname"])
	}

	dnsMap := cConfig.Labels["oops.dns.mappings"]
	if !strings.Contains(dnsMap, "mysql1.b9tpe.tunnel.oops=172.18.0.10") {
		t.Errorf("missing dns mapping for mysql1: %s", dnsMap)
	}

	foundCap := false
	for _, cap := range hConfig.CapAdd {
		if cap == "NET_ADMIN" {
			foundCap = true
			break
		}
	}
	if !foundCap {
		t.Errorf("expected NET_ADMIN capability, got %v", hConfig.CapAdd)
	}
}
