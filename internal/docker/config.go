package docker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OopsConfig represents the unified configuration in oops.yml / config.yml
type OopsConfig struct {
	Registries map[string]string   `yaml:"registries,omitempty"`
	Aliases    map[string]string   `yaml:"aliases,omitempty"` // alias for registries
	Groups     map[string][]string `yaml:"groups,omitempty"`
}

// LoadOopsConfig finds and parses oops.yml / config.yml / groups.yml
func LoadOopsConfig(workDir string) (*OopsConfig, error) {
	cfg := &OopsConfig{
		Registries: make(map[string]string),
		Aliases:    make(map[string]string),
		Groups:     make(map[string][]string),
	}

	candidates := []string{
		filepath.Join(workDir, "stacks", "oops.yml"),
		filepath.Join(workDir, "stacks", "oops.yaml"),
		filepath.Join(workDir, "oops.yml"),
		filepath.Join(workDir, "oops.yaml"),
		filepath.Join(workDir, "stacks", "config.yml"),
		filepath.Join(workDir, "stacks", "config.yaml"),
		filepath.Join(workDir, "config.yml"),
		filepath.Join(workDir, "config.yaml"),
		filepath.Join(workDir, "stacks", "groups.yml"),
		filepath.Join(workDir, "stacks", "groups.yaml"),
		filepath.Join(workDir, "groups.yml"),
		filepath.Join(workDir, "groups.yaml"),
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

			// Merge groups
			for k, v := range fileCfg.Groups {
				if _, exists := cfg.Groups[k]; !exists {
					cfg.Groups[k] = v
				}
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

// LoadGroups is a helper to load only the groups map
func LoadGroups(workDir string) (map[string][]string, error) {
	cfg, err := LoadOopsConfig(workDir)
	if err != nil {
		return nil, err
	}
	return cfg.Groups, nil
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

// GetDefaultGroup reads OOPS_DEFAULT_GROUP from environment or .env file in workDir
func GetDefaultGroup(workDir string) string {
	if val := os.Getenv("OOPS_DEFAULT_GROUP"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}

	envPath := filepath.Join(workDir, ".env")
	file, err := os.Open(envPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if strings.TrimSpace(parts[0]) == "OOPS_DEFAULT_GROUP" {
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			return val
		}
	}

	return ""
}
