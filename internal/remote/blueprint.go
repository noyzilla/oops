package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/box"
)

const defaultCommonExample = `# ==============================================================================
# Oops Common Service Extensions Blueprint (common.yaml.example)
# ==============================================================================
services:
  common-logging:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
`

const defaultComposeExample = `# ==============================================================================
# Oops Base Manifest Blueprint (compose.yaml.example)
# ==============================================================================
# Defines stack/service auto-start order (x-oops-stack) and default network.
# Services or stacks omitted from x-oops-stack will not auto-start on 'oops up'.
# ==============================================================================
# name: oops

x-oops-stack:
  # - /edge
  # - /db
  # - /storage
  # - /tools
  # - /db/mysql
  # - /db/redis
  # - /crm

networks:
  default:
    name: oops_default
    ipam:
      config:
        - subnet: 10.240.0.0/24
          ip_range: 10.240.0.128/25
          gateway: 10.240.0.1
`

const defaultOopsExample = `# ==============================================================================
# Oops System-Level Orchestrator Settings Blueprint (oops.yaml.example)
# ==============================================================================
# Pure configuration file for the Oops CLI and Orchestrator daemon.
# ==============================================================================
x-oops:
  secret: ${OOPS_SECRET:-}

  db:
    mysql_password: ${OOPS_DB_MYSQL_PASSWORD:-}
    postgres_password: ${OOPS_DB_POSTGRES_PASSWORD:-}
    redis_password: ${OOPS_DB_REDIS_PASSWORD:-}

  orchestrator:
    stop_timeout: 30s
    healthcheck:
      timeout: 10m
      interval: 3s

  backup:
    dir: ./backups
    retention: 7d
    data: []

  network:
    subnet: 10.240.0.0/16

  dns:
    tld: oops
    upstreams:
      - 1.1.1.1:53
      - 8.8.8.8:53
    records: []

  volume:
    mount_prefixes: [/mnt, /media, /Volumes]

  registries:
    # my_org: ghcr.io/my_org
    # gar: asia-southeast1-docker.pkg.dev/my-project/my-repo
    # ecr: 123456789.dkr.ecr.ap-southeast-1.amazonaws.com
`

const defaultOopsAllowExample = `# ==============================================================================
# Oops Capsule Synchronization Whitelist (.oopsallow)
# ==============================================================================
/.env
/compose.yaml
/oops.yaml
/common.yaml
/compose*.yaml
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
		{"common.yaml", "common.yaml", defaultCommonExample},
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

		if item.targetName == "oops.yaml" {
			// Enforce Remote Identity Invariant: tld == <remote_name>
			contentStr := string(content)
			if strings.Contains(contentStr, "tld: oops") {
				contentStr = strings.Replace(contentStr, "tld: oops", fmt.Sprintf("tld: %s", remoteName), 1)
				content = []byte(contentStr)
			}
		}

		if err := os.WriteFile(targetPath, content, 0644); err != nil {
			return created, fmt.Errorf("failed to create %s: %w", targetPath, err)
		}
		created = append(created, item.targetName)
	}

	// Initialize standard Oopsbox directory hierarchy (config, data, backups)
	standardDirs := []string{"config", "data", "backups"}
	for _, dirName := range standardDirs {
		dirPath := filepath.Join(capsule, dirName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return created, fmt.Errorf("failed to create capsule %s dir %s: %w", dirName, dirPath, err)
		}
	}

	// Initialize .env with strong credentials if missing
	envPath := filepath.Join(capsule, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// Try to read .env.example from workspace root or oopsbox/.env.example
		var exampleEnvData []byte
		for _, ep := range []string{filepath.Join(workDir, ".env.example"), filepath.Join(workDir, "oopsbox", ".env.example")} {
			if data, err := os.ReadFile(ep); err == nil && len(data) > 0 {
				exampleEnvData = data
				break
			}
		}

		// Write .env.example into capsule temporarily if needed for InitWorkspaceEnv or interpolate directly
		if len(exampleEnvData) > 0 {
			_ = os.WriteFile(filepath.Join(capsule, ".env.example"), exampleEnvData, 0644)
		}

		if err := box.InitWorkspaceEnv(capsule); err != nil {
			return created, fmt.Errorf("failed initializing capsule .env: %w", err)
		}
		// Clean up temporary .env.example in capsule
		_ = os.Remove(filepath.Join(capsule, ".env.example"))

		created = append(created, ".env")
	}

	return created, nil
}
