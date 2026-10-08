package docker

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeConfig represents the root structure of a docker-compose file
type ComposeConfig struct {
	Version       string                    `yaml:"version,omitempty"`
	Services      map[string]ComposeService `yaml:"services"`
	OopsDependsOn []string                  `yaml:"x-oops-depends_on,omitempty"`
}

// ComposeService represents a service inside docker-compose.yml
type ComposeService struct {
	Image         string            `yaml:"image,omitempty"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Labels        yaml.Node         `yaml:"labels,omitempty"`
	ParsedLabels  map[string]string `yaml:"-"`
}

// ParseComposeFile reads and parses a docker-compose.yml file
func ParseComposeFile(filePath string) (*ComposeConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", filePath, err)
	}

	var config ComposeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse compose yaml %s: %w", filePath, err)
	}

	for name, s := range config.Services {
		s.ParsedLabels = parseLabelsNode(&s.Labels)
		config.Services[name] = s
	}

	return &config, nil
}

// parseLabelsNode converts yaml node (list or map) to map[string]string
func parseLabelsNode(node *yaml.Node) map[string]string {
	labels := make(map[string]string)
	if node == nil || node.Kind == 0 {
		return labels
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1].Value
			labels[key] = val
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			val := item.Value
			parts := strings.SplitN(val, "=", 2)
			if len(parts) == 2 {
				labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			} else if len(parts) == 1 {
				labels[strings.TrimSpace(parts[0])] = ""
			}
		}
	}

	return labels
}
