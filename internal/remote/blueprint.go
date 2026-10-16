package remote

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultComposeExample = `# ==============================================================================
# Oops Remote Base Manifest Blueprint (compose.yaml.example)
# ==============================================================================
# Minimal infrastructure skeleton used to initialize /@<remote>/compose.yaml
# Defines lifecycle service groups and shared infrastructure networks.
# ==============================================================================
# name: oops

# Targetable service groups for batch operations (e.g. oops up +db, oops stop +edge)
# Use service name directly (e.g. caddy-proxy, mysql) or project/stack with leading slash (e.g. /tools)
x-oops-groups:
  edge:
    - caddy-proxy
  db:
    - mysql
    - postgres
    - redis
  # lab:
  #   - caddy-proxy
  #   - mysql
  #   - /tools
  #   - /tools2/redis

# Runtime dependencies: services that must be auto-started along with this stack
# (use name for services/, or /<stack>/<service> / /<stack> for cross-stack)
# x-oops-depends:
#   - mysql
#   - /tools2/redis

# Service templates or stacks bundled into compose chain without auto-starting
# x-oops-services:
#   - mailpit
#   - /tools

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
services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
    container_name: oops
    hostname: oops.oops
    restart: unless-stopped
    entrypoint: [ "oops" ]
    command: [ "server" ]
    environment:
      OOPS_SECRET: ${OOPS_SECRET:?OOPS_SECRET must be set}
      OOPS_DB_MYSQL_PASSWORD: ${OOPS_DB_MYSQL_PASSWORD:-}
      OOPS_DB_POSTGRES_PASSWORD: ${OOPS_DB_POSTGRES_PASSWORD:-}
      OOPS_DB_REDIS_PASSWORD: ${OOPS_DB_REDIS_PASSWORD:-}
    labels:
      caddy: "${OOPS_DOMAIN:-oops.web.oops}"
      caddy.reverse_proxy: "{{upstreams 8080}}"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./:/oopsbox
    working_dir: /oopsbox
    networks:
      - oops_edge
    healthcheck:
      test: [ "CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://127.0.0.1:8080/healthz" ]
      interval: 10s
      timeout: 3s
      start_period: 3s
      retries: 3

networks:
  oops_edge:
    external: true

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
