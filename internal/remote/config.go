package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TunnelManifest defines tunnel service configuration in remote.yaml
type TunnelManifest struct {
	Services []string `yaml:"services,omitempty"`
}

// Config represents the remote connection manifest in /@<name>/remote.yaml
type Config struct {
	Host    string         `yaml:"host"`
	Path    string         `yaml:"path"`
	Zone    string         `yaml:"zone,omitempty"`
	Project string         `yaml:"project,omitempty"`
	Tunnel  TunnelManifest `yaml:"tunnel,omitempty"`
}

// CapsuleDir returns the directory path for a remote capsule (e.g. /path/to/workdir/@prod)
func CapsuleDir(workDir, remoteName string) string {
	remoteName = strings.TrimPrefix(remoteName, "@")
	return filepath.Join(workDir, "@"+remoteName)
}

// ConfigPath returns the path to remote.yaml for a given remote
func ConfigPath(workDir, remoteName string) string {
	return filepath.Join(CapsuleDir(workDir, remoteName), "remote.yaml")
}

// LoadConfig reads and parses remote.yaml from the capsule directory
func LoadConfig(workDir, remoteName string) (*Config, error) {
	remoteName = strings.TrimPrefix(remoteName, "@")
	cfgPath := ConfigPath(workDir, remoteName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		// Fallback check: check without @ prefix
		legacyPath := filepath.Join(workDir, remoteName, "remote.yaml")
		data, err = os.ReadFile(legacyPath)
		if err != nil {
			return nil, fmt.Errorf("remote configuration not found for @%s (checked %s)", remoteName, cfgPath)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", cfgPath, err)
	}

	if cfg.Path == "" {
		cfg.Path = "~/oopsbox"
	}

	return &cfg, nil
}

// SaveConfig creates the capsule directory if needed and writes remote.yaml
func SaveConfig(workDir, remoteName string, cfg *Config) error {
	remoteName = strings.TrimPrefix(remoteName, "@")
	capsule := CapsuleDir(workDir, remoteName)
	if err := os.MkdirAll(capsule, 0755); err != nil {
		return fmt.Errorf("failed creating capsule directory %s: %w", capsule, err)
	}

	if cfg.Path == "" {
		cfg.Path = "~/oopsbox"
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal remote config: %w", err)
	}

	cfgPath := ConfigPath(workDir, remoteName)
	return os.WriteFile(cfgPath, data, 0644)
}

// ListRemotes returns all registered remote names by scanning /@*/remote.yaml
func ListRemotes(workDir string) ([]string, error) {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}

	var remotes []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "@") {
			rName := strings.TrimPrefix(name, "@")
			cfgFile := filepath.Join(workDir, name, "remote.yaml")
			if _, err := os.Stat(cfgFile); err == nil {
				remotes = append(remotes, rName)
			}
		}
	}
	return remotes, nil
}

// RemoveRemote deletes the remote capsule directory /@<name>/
func RemoveRemote(workDir, remoteName string) error {
	remoteName = strings.TrimPrefix(remoteName, "@")
	capsule := CapsuleDir(workDir, remoteName)
	if _, err := os.Stat(capsule); os.IsNotExist(err) {
		return fmt.Errorf("remote @%s does not exist", remoteName)
	}
	return os.RemoveAll(capsule)
}
