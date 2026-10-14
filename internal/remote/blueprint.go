package remote

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultComposeExample = `# ==============================================================================
# Oops Remote Base Manifest Blueprint (compose.yaml.example)
# ==============================================================================
name: oops

networks:
  default:
    name: oops_default
  oops_edge:
    name: oops_edge
    driver: bridge
  oops_db:
    name: oops_db
    driver: bridge
  oops_storage:
    name: oops_storage
    driver: bridge
`

const defaultOopsExample = `# ==============================================================================
# Oops Orchestrator Compose Manifest Blueprint (oops.yaml.example)
# ==============================================================================
name: oops-core

services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
    container_name: oops-daemon
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - OOPS_SECRET=${OOPS_SECRET}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./:/workspace:ro
    networks:
      - default

networks:
  default:
    name: oops_default

x-oops:
  registries:
    noyzilla: ghcr.io/noyzilla
  backups:
    retention: 7d
  dns:
    tld: oops
    upstreams:
      - 1.1.1.1:53
      - 8.8.8.8:53
  volume:
    mount_prefixes: [/mnt, /media, /Volumes]
`

const defaultOopsAllowExample = `# ==============================================================================
# Oops Capsule Synchronization Whitelist (.oopsallow)
# ==============================================================================
/.env
/compose.yaml
/oops.yaml
/compose*.local.yaml
/config/**
`

// InitCapsuleBlueprints initializes missing capsule manifests from .example files or embedded blueprints.
func InitCapsuleBlueprints(workDir, remoteName string) ([]string, error) {
	capsule := CapsuleDir(workDir, remoteName)
	if err := os.MkdirAll(capsule, 0755); err != nil {
		return nil, fmt.Errorf("failed creating capsule dir %s: %w", capsule, err)
	}

	type blueprintItem struct {
		targetName  string
		exampleName string
		defaultData string
	}

	items := []blueprintItem{
		{"compose.yaml", "compose.yaml.example", defaultComposeExample},
		{"oops.yaml", "oops.yaml.example", defaultOopsExample},
		{".oopsallow", ".oopsallow.example", defaultOopsAllowExample},
	}

	var created []string

	for _, item := range items {
		targetPath := filepath.Join(capsule, item.targetName)
		if _, err := os.Stat(targetPath); err == nil {
			continue // Already exists, do not overwrite
		}

		// Try to read from workspace .example or oopsbox/.example
		examplePaths := []string{
			filepath.Join(workDir, item.exampleName),
			filepath.Join(workDir, "oopsbox", item.exampleName),
		}

		var content []byte
		for _, ep := range examplePaths {
			if data, err := os.ReadFile(ep); err == nil && len(data) > 0 {
				content = data
				break
			}
		}

		if len(content) == 0 {
			content = []byte(item.defaultData)
		}

		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return created, fmt.Errorf("failed to create %s: %w", targetPath, err)
		}
		created = append(created, item.targetName)
	}

	return created, nil
}
