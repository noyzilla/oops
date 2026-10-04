package docker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GroupsConfig represents the schema of groups.yml
type GroupsConfig struct {
	Groups map[string][]string `yaml:"groups"`
}

// LoadGroups finds and parses groups.yml / groups.yaml from stacks/ or workDir root
func LoadGroups(workDir string) (map[string][]string, error) {
	candidates := []string{
		filepath.Join(workDir, "stacks", "groups.yml"),
		filepath.Join(workDir, "stacks", "groups.yaml"),
		filepath.Join(workDir, "groups.yml"),
		filepath.Join(workDir, "groups.yaml"),
	}

	var foundPath string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			foundPath = c
			break
		}
	}

	if foundPath == "" {
		return make(map[string][]string), nil
	}

	data, err := os.ReadFile(foundPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read groups file %s: %w", foundPath, err)
	}

	var cfg GroupsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse groups file %s: %w", foundPath, err)
	}

	if cfg.Groups == nil {
		cfg.Groups = make(map[string][]string)
	}

	return cfg.Groups, nil
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
