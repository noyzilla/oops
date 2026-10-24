package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DataBackupTarget defines a named data backup volume or filesystem target
type DataBackupTarget struct {
	Name  string   `yaml:"name"`
	Path  string   `yaml:"path,omitempty"`
	Paths []string `yaml:"paths,omitempty"`
}

// GetPaths returns all designated paths for this data backup target
func (d *DataBackupTarget) GetPaths() []string {
	var res []string
	if d.Path != "" {
		res = append(res, d.Path)
	}
	res = append(res, d.Paths...)
	return res
}

// BackupConfig defines backup directory, retention, and data backup targets
type BackupConfig struct {
	Dir       string             `yaml:"dir,omitempty"`
	Retention string             `yaml:"retention,omitempty"`
	Data      []DataBackupTarget `yaml:"data,omitempty"`
}

// DNSConfig defines upstream DNS relays and shared static DNS records in oops.yaml
type DNSConfig struct {
	TLD       string   `yaml:"tld,omitempty"`
	Upstreams []string `yaml:"upstreams,omitempty"`
	Upstream  string   `yaml:"upstream,omitempty"`
	Records   []string `yaml:"records,omitempty"`
}

// GetUpstreams returns the list of upstream DNS servers
func (d *DNSConfig) GetUpstreams() []string {
	if len(d.Upstreams) > 0 {
		return d.Upstreams
	}
	if d.Upstream != "" {
		var res []string
		for _, part := range strings.Split(d.Upstream, ",") {
			if s := strings.TrimSpace(part); s != "" {
				res = append(res, s)
			}
		}
		return res
	}
	return nil
}

// DBConfig defines database credentials in oops.yaml
type DBConfig struct {
	MySQLPassword    string `yaml:"mysql_password,omitempty"`
	PostgresPassword string `yaml:"postgres_password,omitempty"`
	RedisPassword    string `yaml:"redis_password,omitempty"`
}

// HealthcheckConfig defines healthcheck timeout and polling interval
type HealthcheckConfig struct {
	Timeout  string `yaml:"timeout,omitempty"`
	Interval string `yaml:"interval,omitempty"`
}

// OrchestratorConfig defines container lifecycle and healthcheck settings
type OrchestratorConfig struct {
	StopTimeout string            `yaml:"stop_timeout,omitempty"`
	Healthcheck HealthcheckConfig `yaml:"healthcheck,omitempty"`
}

// VolumeConfig defines persistent volume guard settings in oops.yaml
type VolumeConfig struct {
	MountPrefixes []string `yaml:"mount_prefixes,omitempty"`
}

// NetworkConfig defines mesh subnet and IPAM settings in oops.yaml
type NetworkConfig struct {
	Subnet string `yaml:"subnet,omitempty"`
}

type OopsConfig struct {
	Secret       string             `yaml:"secret,omitempty"`
	DB           DBConfig           `yaml:"db,omitempty"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator,omitempty"`
	Backup       BackupConfig       `yaml:"backup,omitempty"`
	Backups      BackupConfig       `yaml:"backups,omitempty"` // Alias for backward compatibility
	DNS          DNSConfig          `yaml:"dns,omitempty"`
	Network      NetworkConfig      `yaml:"network,omitempty"`
	Volume       VolumeConfig       `yaml:"volume,omitempty"`
	Registries   map[string]string  `yaml:"registries,omitempty"`
}

// LoadOopsConfig finds and parses oops.yaml
func LoadOopsConfig(workDir string) (*OopsConfig, error) {
	cfg := &OopsConfig{
		Registries: make(map[string]string),
	}

	candidates := []string{
		filepath.Join(workDir, "oops.yaml"),
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			data, err := os.ReadFile(c)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file %s: %w", c, err)
			}

			var wrapper struct {
				XOops      *OopsConfig `yaml:"x-oops"`
				OopsConfig `yaml:",inline"`
			}
			if err := yaml.Unmarshal(data, &wrapper); err != nil {
				return nil, fmt.Errorf("failed to parse yaml in %s: %w", c, err)
			}

			fileCfg := wrapper.OopsConfig
			if wrapper.XOops != nil {
				if wrapper.XOops.Secret != "" {
					fileCfg.Secret = wrapper.XOops.Secret
				}
				if wrapper.XOops.DB.MySQLPassword != "" {
					fileCfg.DB.MySQLPassword = wrapper.XOops.DB.MySQLPassword
				}
				if wrapper.XOops.DB.PostgresPassword != "" {
					fileCfg.DB.PostgresPassword = wrapper.XOops.DB.PostgresPassword
				}
				if wrapper.XOops.DB.RedisPassword != "" {
					fileCfg.DB.RedisPassword = wrapper.XOops.DB.RedisPassword
				}
				if wrapper.XOops.Orchestrator.StopTimeout != "" {
					fileCfg.Orchestrator.StopTimeout = wrapper.XOops.Orchestrator.StopTimeout
				}
				if wrapper.XOops.Orchestrator.Healthcheck.Timeout != "" {
					fileCfg.Orchestrator.Healthcheck.Timeout = wrapper.XOops.Orchestrator.Healthcheck.Timeout
				}
				if wrapper.XOops.Orchestrator.Healthcheck.Interval != "" {
					fileCfg.Orchestrator.Healthcheck.Interval = wrapper.XOops.Orchestrator.Healthcheck.Interval
				}
				if wrapper.XOops.Backup.Dir != "" {
					fileCfg.Backup.Dir = wrapper.XOops.Backup.Dir
				}
				if wrapper.XOops.Backup.Retention != "" {
					fileCfg.Backup.Retention = wrapper.XOops.Backup.Retention
				}
				if len(wrapper.XOops.Backup.Data) > 0 {
					fileCfg.Backup.Data = wrapper.XOops.Backup.Data
				}
				// Support legacy backups: key if backup: was not set
				if wrapper.XOops.Backups.Dir != "" && fileCfg.Backup.Dir == "" {
					fileCfg.Backup.Dir = wrapper.XOops.Backups.Dir
				}
				if wrapper.XOops.Backups.Retention != "" && fileCfg.Backup.Retention == "" {
					fileCfg.Backup.Retention = wrapper.XOops.Backups.Retention
				}
				if len(wrapper.XOops.Backups.Data) > 0 && len(fileCfg.Backup.Data) == 0 {
					fileCfg.Backup.Data = wrapper.XOops.Backups.Data
				}
				if wrapper.XOops.DNS.TLD != "" {
					fileCfg.DNS.TLD = wrapper.XOops.DNS.TLD
				}
				if wrapper.XOops.Network.Subnet != "" {
					fileCfg.Network.Subnet = wrapper.XOops.Network.Subnet
				}
				if len(wrapper.XOops.DNS.Upstreams) > 0 {
					fileCfg.DNS.Upstreams = wrapper.XOops.DNS.Upstreams
				}
				if wrapper.XOops.DNS.Upstream != "" {
					fileCfg.DNS.Upstream = wrapper.XOops.DNS.Upstream
				}
				if len(wrapper.XOops.DNS.Records) > 0 {
					fileCfg.DNS.Records = wrapper.XOops.DNS.Records
				}
				if wrapper.XOops.Volume.MountPrefixes != nil {
					fileCfg.Volume.MountPrefixes = wrapper.XOops.Volume.MountPrefixes
				}
				if len(wrapper.XOops.Registries) > 0 {
					fileCfg.Registries = wrapper.XOops.Registries
				}
			}

			// Assign file values to cfg
			cfg.Secret = fileCfg.Secret
			cfg.DB = fileCfg.DB
			cfg.Orchestrator = fileCfg.Orchestrator
			cfg.Backup = fileCfg.Backup
			if cfg.Backup.Dir == "" && fileCfg.Backups.Dir != "" {
				cfg.Backup.Dir = fileCfg.Backups.Dir
			}
			if cfg.Backup.Retention == "" && fileCfg.Backups.Retention != "" {
				cfg.Backup.Retention = fileCfg.Backups.Retention
			}
			if len(cfg.Backup.Data) == 0 && len(fileCfg.Backups.Data) > 0 {
				cfg.Backup.Data = fileCfg.Backups.Data
			}
			cfg.Backups = cfg.Backup

			for k, v := range fileCfg.Registries {
				if _, exists := cfg.Registries[k]; !exists {
					cfg.Registries[k] = strings.TrimRight(v, "/")
				}
			}

			cfg.DNS = fileCfg.DNS
			cfg.Network = fileCfg.Network
			cfg.Volume = fileCfg.Volume
			break
		}
	}

	// Apply Environment Variable Overrides (x-oops = OOPS_)
	if envSecret := os.Getenv("OOPS_SECRET"); envSecret != "" {
		cfg.Secret = envSecret
	}
	if envPass := os.Getenv("OOPS_DB_MYSQL_PASSWORD"); envPass != "" {
		cfg.DB.MySQLPassword = envPass
	}
	if envPass := os.Getenv("OOPS_DB_POSTGRES_PASSWORD"); envPass != "" {
		cfg.DB.PostgresPassword = envPass
	}
	if envPass := os.Getenv("OOPS_DB_REDIS_PASSWORD"); envPass != "" {
		cfg.DB.RedisPassword = envPass
	}

	// Orchestrator overrides
	if envStop := os.Getenv("OOPS_STOP_TIMEOUT"); envStop != "" {
		cfg.Orchestrator.StopTimeout = envStop
	}
	if envHkTimeout := os.Getenv("OOPS_HEALTHCHECK_TIMEOUT"); envHkTimeout != "" {
		cfg.Orchestrator.Healthcheck.Timeout = envHkTimeout
	}
	if envHkInterval := os.Getenv("OOPS_HEALTHCHECK_INTERVAL"); envHkInterval != "" {
		cfg.Orchestrator.Healthcheck.Interval = envHkInterval
	}

	// Backup overrides
	if envBackupDir := os.Getenv("OOPS_BACKUP_DIR"); envBackupDir != "" {
		cfg.Backup.Dir = envBackupDir
		cfg.Backups.Dir = envBackupDir
	}
	if envBackupRetention := os.Getenv("OOPS_BACKUP_RETENTION"); envBackupRetention != "" {
		cfg.Backup.Retention = envBackupRetention
		cfg.Backups.Retention = envBackupRetention
	}

	// Registries overrides
	if envAliases := os.Getenv("OOPS_REGISTRY_ALIASES"); envAliases != "" {
		pairs := strings.Split(envAliases, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimRight(strings.TrimSpace(parts[1]), "/")
				if k != "" && v != "" {
					cfg.Registries[k] = v
				}
			}
		}
	}

	// DNS overrides
	if envTLD := os.Getenv("OOPS_DNS_TLD"); envTLD != "" {
		cfg.DNS.TLD = envTLD
	}
	if envUpstreams := os.Getenv("OOPS_DNS_UPSTREAMS"); envUpstreams != "" {
		cfg.DNS.Upstream = envUpstreams
		cfg.DNS.Upstreams = strings.Split(envUpstreams, ",")
	} else if envUpstream := os.Getenv("OOPS_DNS_UPSTREAM"); envUpstream != "" {
		cfg.DNS.Upstream = envUpstream
		cfg.DNS.Upstreams = strings.Split(envUpstream, ",")
	}
	if envRecords := os.Getenv("OOPS_DNS_RECORDS"); envRecords != "" {
		for _, r := range strings.Split(envRecords, ",") {
			if s := strings.TrimSpace(r); s != "" {
				cfg.DNS.Records = append(cfg.DNS.Records, s)
			}
		}
	}

	// Volume mount prefixes override
	if envPrefixes := os.Getenv("OOPS_VOLUME_MOUNT_PREFIXES"); envPrefixes != "" {
		var prefixes []string
		for _, p := range strings.Split(envPrefixes, ",") {
			if s := strings.TrimSpace(p); s != "" {
				prefixes = append(prefixes, s)
			}
		}
		cfg.Volume.MountPrefixes = prefixes
	}

	// Network overrides
	if envSubnet := os.Getenv("OOPS_NETWORK_SUBNET"); envSubnet != "" {
		cfg.Network.Subnet = envSubnet
	}

	// Apply default values
	if cfg.DNS.TLD == "" {
		cfg.DNS.TLD = "oops"
	}
	if cfg.Orchestrator.StopTimeout == "" {
		cfg.Orchestrator.StopTimeout = "30s"
	}
	if cfg.Orchestrator.Healthcheck.Timeout == "" {
		cfg.Orchestrator.Healthcheck.Timeout = "10m"
	}
	if cfg.Orchestrator.Healthcheck.Interval == "" {
		cfg.Orchestrator.Healthcheck.Interval = "3s"
	}
	if cfg.Backup.Dir == "" {
		cfg.Backup.Dir = "./backups"
		cfg.Backups.Dir = "./backups"
	}
	if cfg.Backup.Retention == "" {
		cfg.Backup.Retention = "7d"
		cfg.Backups.Retention = "7d"
	}

	return cfg, nil
}


// ExpandImageAlias expands a prefix alias (e.g. gar/my-app:v1.0 -> asia-southeast1-docker.pkg.dev/.../my-app:v1.0)
func ExpandImageAlias(rawImage string, registries map[string]string) string {
	trimmed := strings.TrimSpace(rawImage)
	if trimmed == "" || len(registries) == 0 {
		return trimmed
	}

	// Check if rawImage starts with "image:" or "img:"
	if strings.HasPrefix(trimmed, "image:") {
		trimmed = strings.TrimPrefix(trimmed, "image:")
	} else if strings.HasPrefix(trimmed, "img:") {
		trimmed = strings.TrimPrefix(trimmed, "img:")
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 {
		aliasKey := parts[0]
		subPath := parts[1]
		if registryBase, ok := registries[aliasKey]; ok {
			return registryBase + "/" + subPath
		}
	}

	return trimmed
}

// ValidateNetworkSubnet validates that a CIDR string is a valid IPv4 network block suitable for Oops Mesh.
// It checks IPv4 format, prevents loopback ranges (127.0.0.0/8), and validates prefix length.
func ValidateNetworkSubnet(cidr string) error {
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return nil
	}
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid network subnet CIDR %q: %w", cidr, err)
	}
	if ip.To4() == nil {
		return fmt.Errorf("network subnet must be IPv4, got %q", cidr)
	}
	if ip.IsLoopback() {
		return fmt.Errorf("loopback subnet %q (127.0.0.0/8) cannot be used for bridge networking", cidr)
	}
	ones, _ := ipNet.Mask.Size()
	if ones < 8 || ones > 28 {
		return fmt.Errorf("network subnet prefix /%d must be between /8 and /28", ones)
	}
	return nil
}

// IsSubnetContained checks if childCIDR is completely contained within parentCIDR.
func IsSubnetContained(parentCIDR, childCIDR string) (bool, error) {
	_, parentNet, err := net.ParseCIDR(strings.TrimSpace(parentCIDR))
	if err != nil {
		return false, fmt.Errorf("invalid parent CIDR %q: %w", parentCIDR, err)
	}
	childIP, childNet, err := net.ParseCIDR(strings.TrimSpace(childCIDR))
	if err != nil {
		return false, fmt.Errorf("invalid child CIDR %q: %w", childCIDR, err)
	}

	if !parentNet.Contains(childIP) {
		return false, nil
	}

	parentOnes, _ := parentNet.Mask.Size()
	childOnes, _ := childNet.Mask.Size()
	if childOnes < parentOnes {
		return false, nil
	}

	return true, nil
}
