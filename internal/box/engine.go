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
	DNS struct {
		TLD string `yaml:"tld"`
	} `yaml:"dns"`
}

// LoadOopsboxConfig reads oopsbox.yml from the workspace directory if present.
func LoadOopsboxConfig(workDir string) (*OopsboxConfig, error) {
	cfg := &OopsboxConfig{}
	cfg.Engine.Type = "auto"
	cfg.DNS.TLD = "oops"

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
	if cfg.DNS.TLD == "" {
		cfg.DNS.TLD = "oops"
	}

	// Environment variable overrides
	if envEngine := os.Getenv("OOPS_ENGINE_TYPE"); envEngine != "" {
		cfg.Engine.Type = envEngine
	}
	if envTLD := os.Getenv("OOPS_DNS_TLD"); envTLD != "" {
		cfg.DNS.TLD = envTLD
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

// GetHostGatewayIP returns the gateway IP for reaching the host from containers.
func GetHostGatewayIP() string {
	// If OrbStack or Docker Desktop on Mac
	if runtime.GOOS == "darwin" {
		return "192.168.215.1"
	}
	return "172.17.0.1"
}

// SetupMacOSResolver configures /etc/resolver/<tld> on macOS.
func SetupMacOSResolver(tld string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	resolverDir := "/etc/resolver"
	resolverFile := filepath.Join(resolverDir, tld)

	content := "nameserver 127.0.0.1\nport 53\n"
	current, err := os.ReadFile(resolverFile)
	if err == nil && string(current) == content {
		return nil // already configured
	}

	fmt.Printf("==> Configuring macOS DNS resolver for .%s domain (/etc/resolver/%s)...\n", tld, tld)
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

// IsDockerRunning verifies if Docker daemon responds.
func IsDockerRunning() bool {
	var out bytes.Buffer
	cmd := exec.Command("docker", "info")
	cmd.Stdout = &out
	cmd.Stderr = &out
	return cmd.Run() == nil
}
