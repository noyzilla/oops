package box

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type OopsboxConfig struct {
	Engine struct {
		Type   string `yaml:"type"`
		CPU    int    `yaml:"cpu"`
		Memory string `yaml:"memory"`
		Disk   string `yaml:"disk"`
	} `yaml:"engine"`
}

// LoadOopsboxConfig reads oopsbox.yml from the workspace directory if present.
func LoadOopsboxConfig(workDir string) (*OopsboxConfig, error) {
	cfg := &OopsboxConfig{}
	cfg.Engine.Type = "auto"

	cfgPath := filepath.Join(workDir, "oopsbox.yml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse oopsbox.yml: %w", err)
	}

	if cfg.Engine.Type == "" {
		cfg.Engine.Type = "auto"
	}

	// Environment variable overrides
	if envEngine := os.Getenv("OOPS_ENGINE_TYPE"); envEngine != "" {
		cfg.Engine.Type = envEngine
	}

	return cfg, nil
}

// EnsureEngineStarted ensures that the designated or auto-detected container engine is active.
func EnsureEngineStarted(cfg *OopsboxConfig) (string, error) {
	engineType := strings.ToLower(strings.TrimSpace(cfg.Engine.Type))
	if engineType == "" || engineType == "auto" {
		engineType = DetectBestEngine()
	}

	switch engineType {
	case "orbstack":
		if runtime.GOOS != "darwin" {
			return "docker", nil
		}
		if _, err := exec.LookPath("orb"); err != nil {
			return "docker", nil
		}
		cmd := exec.Command("orb", "status")
		if err := cmd.Run(); err != nil {
			fmt.Println("==> Starting OrbStack engine...")
			startCmd := exec.Command("orb", "start")
			startCmd.Stdout = os.Stdout
			startCmd.Stderr = os.Stderr
			if err := startCmd.Run(); err != nil {
				return "orbstack", fmt.Errorf("failed to start OrbStack: %w", err)
			}
		}
		return "orbstack", nil

	case "colima":
		if _, err := exec.LookPath("colima"); err != nil {
			return "docker", nil
		}
		cmd := exec.Command("colima", "status")
		if err := cmd.Run(); err != nil {
			fmt.Println("==> Starting Colima VM engine...")
			args := []string{"start"}
			if cfg.Engine.CPU > 0 {
				args = append(args, "--cpu", fmt.Sprintf("%d", cfg.Engine.CPU))
			}
			if cfg.Engine.Memory != "" {
				args = append(args, "--memory", cfg.Engine.Memory)
			}
			if cfg.Engine.Disk != "" {
				args = append(args, "--disk", cfg.Engine.Disk)
			}
			if runtime.GOOS == "darwin" {
				args = append(args, "--vm-type=vz")
			}
			startCmd := exec.Command("colima", args...)
			startCmd.Stdout = os.Stdout
			startCmd.Stderr = os.Stderr
			if err := startCmd.Run(); err != nil {
				return "colima", fmt.Errorf("failed to start Colima: %w", err)
			}
		}
		return "colima", nil

	case "docker":
		cmd := exec.Command("docker", "info")
		if err := cmd.Run(); err != nil {
			return "docker", fmt.Errorf("Docker daemon is not running. Please start Docker Engine / Docker Desktop")
		}
		return "docker", nil

	default:
		return "docker", nil
	}
}

// DetectBestEngine inspects the local host environment and determines the best engine available.
func DetectBestEngine() string {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("orb"); err == nil {
			return "orbstack"
		}
		if _, err := exec.LookPath("colima"); err == nil {
			return "colima"
		}
	}
	return "docker"
}

// GetHostGatewayIP dynamically discovers the host gateway IP address from Docker networks.
func GetHostGatewayIP() string {
	// 1. Try inspecting gateway from running oops container
	cmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.Gateway}}{{break}}{{end}}", "oops")
	if out, err := cmd.Output(); err == nil {
		gw := strings.TrimSpace(string(out))
		if gw != "" && gw != "<no value>" {
			return gw
		}
	}

	// 2. Try inspecting gateway from net-edge network
	netCmd := exec.Command("docker", "network", "inspect", "net-edge", "-f", "{{range .IPAM.Config}}{{.Gateway}}{{break}}{{end}}")
	if out, err := netCmd.Output(); err == nil {
		gw := strings.TrimSpace(string(out))
		if gw != "" && gw != "<no value>" {
			return gw
		}
	}

	// 3. Try inspecting standard bridge network
	bridgeCmd := exec.Command("docker", "network", "inspect", "bridge", "-f", "{{range .IPAM.Config}}{{.Gateway}}{{break}}{{end}}")
	if out, err := bridgeCmd.Output(); err == nil {
		gw := strings.TrimSpace(string(out))
		if gw != "" && gw != "<no value>" {
			return gw
		}
	}

	// 4. Fallback default
	return "172.17.0.1"
}

// GetOopsContainerIP inspects the running oops container and retrieves its network IP address.
func GetOopsContainerIP() string {
	cmd := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{break}}{{end}}", "oops")
	out, err := cmd.Output()
	if err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			return ip
		}
	}
	return ""
}

// SetupMacOSResolver configures /etc/resolver/<tld> on macOS.
func SetupMacOSResolver(tld string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	resolverDir := "/etc/resolver"
	resolverFile := filepath.Join(resolverDir, tld)

	nameserverIP := GetOopsContainerIP()
	if nameserverIP == "" {
		nameserverIP = "127.0.0.1"
	}

	content := fmt.Sprintf("nameserver %s\n", nameserverIP)
	current, err := os.ReadFile(resolverFile)
	if err == nil && string(current) == content {
		return nil // already configured
	}

	fmt.Printf("==> Configuring macOS DNS resolver for .%s domain (nameserver %s)...\n", tld, nameserverIP)
	script := fmt.Sprintf("mkdir -p %s && printf '%s' > %s", resolverDir, content, resolverFile)
	cmd := exec.Command("sudo", "sh", "-c", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SyncStaticDNSRecords writes host.<tld> to data/oops/dns.records in the workspace.
func SyncStaticDNSRecords(workDir, tld string) error {
	dnsDir := filepath.Join(workDir, "data", "oops")
	if err := os.MkdirAll(dnsDir, 0755); err != nil {
		return err
	}
	recordsFile := filepath.Join(dnsDir, "dns.records")
	gatewayIP := GetHostGatewayIP()

	var lines []string
	if existing, err := os.ReadFile(recordsFile); err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "host."+tld) && !strings.HasPrefix(line, "host.oops") {
				lines = append(lines, line)
			}
		}
	}

	lines = append(lines, fmt.Sprintf("host.%s %s", tld, gatewayIP))
	newContent := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(recordsFile, []byte(newContent), 0644)
}

// DiscoverDockerSubnets queries Docker for all bridge network subnets dynamically.
func DiscoverDockerSubnets() []string {
	cmd := exec.Command("docker", "network", "ls", "--filter", "driver=bridge", "--format", "{{.Name}}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var subnets []string
	seen := make(map[string]bool)

	for _, name := range strings.Split(string(out), "\n") {
		name = strings.TrimSpace(name)
		if name == "" || name == "bridge" || name == "host" || name == "none" {
			continue
		}

		inspectCmd := exec.Command("docker", "network", "inspect", name, "-f", "{{range .IPAM.Config}}{{.Subnet}}{{break}}{{end}}")
		if sOut, err := inspectCmd.Output(); err == nil {
			subnet := strings.TrimSpace(string(sOut))
			if subnet != "" && subnet != "<no value>" && !seen[subnet] {
				seen[subnet] = true
				subnets = append(subnets, subnet)
			}
		}
	}
	return subnets
}

// SetupColimaRouting dynamically configures macOS routes and Colima VM NAT for all active bridge networks.
func SetupColimaRouting() error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	if _, err := exec.LookPath("colima"); err != nil {
		return nil
	}
	if err := exec.Command("colima", "status").Run(); err != nil {
		return nil
	}

	// Get Colima VM IP on col0
	out, err := exec.Command("colima", "ssh", "--", "ip", "addr", "show", "col0").Output()
	if err != nil {
		return nil
	}

	var vmIP string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				vmIP = strings.Split(parts[1], "/")[0]
				break
			}
		}
	}
	if vmIP == "" {
		return nil
	}

	// Configure NAT / IP Forwarding inside Colima VM
	natScript := `sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1
if ! sudo iptables -C FORWARD -j ACCEPT 2>/dev/null; then
  sudo iptables -I FORWARD 1 -j ACCEPT
fi
if ! sudo iptables -t nat -C POSTROUTING -j MASQUERADE 2>/dev/null; then
  sudo iptables -t nat -I POSTROUTING 1 -j MASQUERADE
fi`
	_ = exec.Command("colima", "ssh", "--", "sh", "-c", natScript).Run()

	// Add routes for all discovered subnets
	subnets := DiscoverDockerSubnets()
	for _, subnet := range subnets {
		_ = exec.Command("sudo", "route", "-n", "delete", "-net", subnet).Run()
		_ = exec.Command("sudo", "route", "-n", "add", "-net", subnet, vmIP).Run()
	}

	return nil
}

// IsDockerRunning verifies if Docker daemon responds.
func IsDockerRunning() bool {
	var out bytes.Buffer
	cmd := exec.Command("docker", "info")
	cmd.Stdout = &out
	cmd.Stderr = &out
	return cmd.Run() == nil
}
