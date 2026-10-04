package docker

import (
	"fmt"
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

// BackupConfig defines backup retention and data backup directories
type BackupConfig struct {
	Retention string             `yaml:"retention,omitempty"`
	Data      []DataBackupTarget `yaml:"data,omitempty"`
}

// ColimaConfig defines VM resource settings for macOS Colima workstation
type ColimaConfig struct {
	CPU    int    `yaml:"cpu,omitempty"`
	Memory int    `yaml:"memory,omitempty"`
	Disk   int    `yaml:"disk,omitempty"`
	VMType string `yaml:"vm_type,omitempty"`
}

// DNSConfig defines upstream DNS relays and shared static DNS records in oops.yml
type DNSConfig struct {
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

// OopsConfig represents the unified configuration in oops.yml / config.yml
type OopsConfig struct {
	Registries map[string]string   `yaml:"registries,omitempty"`
	Aliases    map[string]string   `yaml:"aliases,omitempty"` // alias for registries
	Profiles   map[string][]string `yaml:"profiles,omitempty"`
	Backups    BackupConfig        `yaml:"backups,omitempty"`
	DNS        DNSConfig           `yaml:"dns,omitempty"`
	Colima     ColimaConfig        `yaml:"colima,omitempty"`
}

// LoadOopsConfig finds and parses oops.yml / config.yml
func LoadOopsConfig(workDir string) (*OopsConfig, error) {
	cfg := &OopsConfig{
		Registries: make(map[string]string),
		Aliases:    make(map[string]string),
		Profiles:   make(map[string][]string),
	}

	candidates := []string{
		filepath.Join(workDir, "oops.yml"),
		filepath.Join(workDir, "oops.yaml"),
		filepath.Join(workDir, "stacks", "oops.yml"),
		filepath.Join(workDir, "stacks", "oops.yaml"),
		filepath.Join(workDir, "config.yml"),
		filepath.Join(workDir, "config.yaml"),
		filepath.Join(workDir, "stacks", "config.yml"),
		filepath.Join(workDir, "stacks", "config.yaml"),
		filepath.Join(workDir, "groups.yml"),
		filepath.Join(workDir, "groups.yaml"),
		filepath.Join(workDir, "stacks", "groups.yml"),
		filepath.Join(workDir, "stacks", "groups.yaml"),
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			data, err := os.ReadFile(c)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file %s: %w", c, err)
			}

			var fileCfg OopsConfig
			if err := yaml.Unmarshal(data, &fileCfg); err != nil {
				return nil, fmt.Errorf("failed to parse yaml in %s: %w", c, err)
			}

			// Merge registries
			for k, v := range fileCfg.Registries {
				if _, exists := cfg.Registries[k]; !exists {
					cfg.Registries[k] = strings.TrimRight(v, "/")
				}
			}
			for k, v := range fileCfg.Aliases {
				if _, exists := cfg.Registries[k]; !exists {
					cfg.Registries[k] = strings.TrimRight(v, "/")
				}
			}

			// Merge profiles
			for k, v := range fileCfg.Profiles {
				if _, exists := cfg.Profiles[k]; !exists {
					cfg.Profiles[k] = v
				}
			}

			// Merge backups
			if fileCfg.Backups.Retention != "" && cfg.Backups.Retention == "" {
				cfg.Backups.Retention = fileCfg.Backups.Retention
			}
			if len(fileCfg.Backups.Data) > 0 && len(cfg.Backups.Data) == 0 {
				cfg.Backups.Data = fileCfg.Backups.Data
			}

			// Merge DNS
			if len(fileCfg.DNS.Upstreams) > 0 && len(cfg.DNS.Upstreams) == 0 {
				cfg.DNS.Upstreams = fileCfg.DNS.Upstreams
			}
			if fileCfg.DNS.Upstream != "" && cfg.DNS.Upstream == "" {
				cfg.DNS.Upstream = fileCfg.DNS.Upstream
			}
			if len(fileCfg.DNS.Records) > 0 && len(cfg.DNS.Records) == 0 {
				cfg.DNS.Records = fileCfg.DNS.Records
			}

			// Merge colima
			if fileCfg.Colima.CPU > 0 && cfg.Colima.CPU == 0 {
				cfg.Colima.CPU = fileCfg.Colima.CPU
			}
			if fileCfg.Colima.Memory > 0 && cfg.Colima.Memory == 0 {
				cfg.Colima.Memory = fileCfg.Colima.Memory
			}
			if fileCfg.Colima.Disk > 0 && cfg.Colima.Disk == 0 {
				cfg.Colima.Disk = fileCfg.Colima.Disk
			}
			if fileCfg.Colima.VMType != "" && cfg.Colima.VMType == "" {
				cfg.Colima.VMType = fileCfg.Colima.VMType
			}
		}
	}

	// Also parse OOPS_REGISTRY_ALIASES from env (e.g. "gar=asia-southeast1-docker.pkg.dev/proj/repo,ghcr=ghcr.io/org")
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

	return cfg, nil
}

// LoadProfiles is a helper to load only the profiles map
func LoadProfiles(workDir string) (map[string][]string, error) {
	cfg, err := LoadOopsConfig(workDir)
	if err != nil {
		return nil, err
	}
	return cfg.Profiles, nil
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
