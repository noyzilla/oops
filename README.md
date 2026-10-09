# Oops & Oopsbox

![Oops Logo](logo.jpg)
> *Because "Oops, sorry!" is the most common phrase when DevOps breaks production.*

**Oops** is a zero-overhead DevOps orchestration engine, native DNS daemon, and multi-stack CLI written in Go.
**Oopsbox** is the flagship turnkey infrastructure blueprint that delivers an instant, production-grade Docker Compose workstation and server environment with automated local HTTPS (`*.web.oops`), embedded DNS resolution (`:53`), and multi-group stack isolation.

---

## Core Philosophy & Why Oops?

Oops and Oopsbox eliminate DevOps complexity through three foundational principles:

- **1:1 Dev-to-Prod Parity (Zero "Works on my machine" Surprises)**:
  - The exact same infrastructure blueprint (`oopsbox/`) runs seamlessly on local workstations (macOS / Linux) and production servers.
  - Standardizes network segmentation (`net-edge`, `net-db`), storage paths (`data/`, `backups/`), and local DNS resolution across all machines.

- **Git-Driven Infrastructure (No More SSH Snowflake Configs)**:
  - Git is the single source of truth for all stack definitions and service configurations (`stacks/<group>/<service>/`).
  - Push config updates or new service versions to Git, and let the Oops Webhook Daemon reconcile, pull, and reload containers automatically without ever SSHing into the server to manually edit configs.

- **Effortless Tech Stack Upgrades (Zero-Downtime Multi-Group Architecture)**:
  - Independent modular groups (`stacks/edge`, `stacks/db`, `stacks/tool`, `stacks/apps`) allow updating language runtimes, database engines, or mock tools with zero blast-radius on unrelated services.
  - Sequential rolling updates (`oops update /apps -d 5s`) ensure zero downtime with pre-stop hooks and automated healthcheck polling.

---

## The Flagship: Oopsbox Blueprint

**Oopsbox** is a complete, pre-configured Infrastructure as Code (IaC) setup located in [`oopsbox/`](oopsbox/README.md). It eliminates DevOps boilerplate and provides an instant developer environment on macOS (OrbStack / Colima) and Linux.

```text
oopsbox/
├── oops.yml              # Master Config: backups, registry shortcuts, DNS
├── oopsbox.yml           # Workstation Settings: VM engine (OrbStack/Colima), resources, local DNS
├── compose.yml           # Root Stack (/.): Core environment and topology relationships (include)
├── compose.edge-caddy.yml# Edge Reverse Proxy Stack (Caddy)
├── compose.db.yml        # Persistence & Cache Stack (MySQL, Postgres, Redis)
├── compose.tool.yml      # Dev & Mock Utilities Stack (httpbin, mailpit)
├── compose.apps.yml      # Application Services Stack (web-app, worker)
├── config/               # Version-controlled service configuration (e.g., caddy/Caddyfile)
├── data/                 # Live container storage (High-IOPS persistent host volumes)
└── backups/              # Automated database dumps & filesystem data archives
```

### Why Oopsbox?

- **1-Command Workstation Setup**: Run `oops box init ~/oopsbox` and `oops box start` to boot the VM, set up host networking, register the macOS/Linux DNS resolver, and trust root SSL certificates.
- **Automated Local HTTPS**: Caddy Edge Proxy automatically issues and serves valid TLS certificates with green locks for `https://*.web.oops`.
- **Embedded Zero-Config DNS**: Containers are instantly resolvable by hostname (e.g. `mysql.oops`, `redis.oops`, `host.oops`, `vm.oops`) through the embedded Oops DNS engine.
- **Strict Network Isolation**: Enforces least privilege across `net-edge` (public ingress) and `net-db` (isolated backend).
- **Single Source of Truth (`oops.yml`)**: Configure project profiles (`profiles:`), image registry aliases (`registries:`), and automated backups in one clean YAML file.

[Read the Full Oopsbox Blueprint Guide](oopsbox/README.md)

---

## The Engine: Oops CLI & Daemon

The `oops` CLI binary manages multi-group containers, performs safe rolling updates, automates database provisioning, and executes backups.

### Smart Target Selectors
```bash
# Start default root stack (/. mapped to compose.yml) and its dependencies
oops up

# Start specific stack
oops up /edge-caddy
oops up /db

# Switch active stack (starts target stack & gracefully stops others)
oops switch /apps

# Double Dot (..) wildcards
oops restart app..          # Matches all services starting with 'app'
oops stop ..worker          # Matches all services ending with 'worker'
oops logs ..api..           # Matches any service containing 'api'

# Stop all services except exclusions
oops stop -x /edge -x redis
```

### Zero-Downtime Sequential Rolling Updates
```bash
# Pull -> Pre-Stop Hook -> Recreate -> Health Check Poll -> Delay Gap
oops update /apps -d 5s
oops update gar/my-app:v2.0
```

### Automated Database & User Provisioning
```bash
# Provision DB & User (1:1 mapped User implicitly created) with a 20-char secure password
oops db mysql create my_database
oops db postgres:pg-replica create analytics_db

# Provision SELECT-only readonly users (mapped as <db_name>__ro_<app_name>)
oops db mysql readonly my_database metabase

# Rotate user password
oops db mysql passwd my_database new_password
```

### Automated Backup & Safe Interactive Restore
```bash
# Full backup (Database Dumps + Filesystem Volume Tarballs)
oops backup
oops backup prune -r 7d

# Safe restore with workspace check, confirmation guard, and dry-run preview
oops restore backups/mysql_backup_20261004.sql.gz
oops restore backups/data_uploads_20261004.tar.gz --dry-run
```

### Native DNS & Container IP Inspection
```bash
# Inspect active DNS resolution table (sorted numerically by IP)
oops dns
oops dns get mysql.oops

# Query individual container IP
oops ip mysql
oops ip caddy

# List all container IPs sorted by IP
oops ips

# Manage custom static / wildcard DNS records
oops dns set api.internal 10.0.0.5
oops dns set .staging.oops 127.0.0.1
oops dns del api.internal
```

### Live Log Streaming
```bash
# Stream live logs across stacks, services, or wildcards (default tail: 50 lines)
oops logs mysql -f
oops logs /edge
oops logs /db --tail 20
oops logs app.. -f
```

### CLI Version & Self-Update
```bash
# Display binary version & build information
oops -v
oops version

# Self-update oops binary directly from GitHub releases
oops selfupdate
oops selfupdate --check
```

---

## CI/CD Webhook Deployments

Oops runs as a lightweight daemon (`oops server`) listening for deployment webhooks from GitHub Actions, GitLab CI, or custom pipelines.

### Container Labels

Add labels to target containers to authorize Oops and configure lifecycle hooks:

| Label | Required | Description |
| :--- | :---: | :--- |
| `oops.enable` | Yes | Must be `"true"` to authorize Oops management. |
| `oops.secret` | Yes | Secure webhook token for per-project authentication. |
| `oops.health.url` | No | HTTP health check URL for polling readiness after recreation. |
| `oops.git.url` | Yes (Git Mode) | Git repository URL for volume source checkout. |
| `oops.git.dir` | Yes (Git Mode) | Absolute path inside container where source is mounted (e.g. `/app`). |

### Webhook Trigger Example

```bash
curl -X POST \
  -H "Authorization: Bearer my-secure-token" \
  -H "Content-Type: application/json" \
  -d '{"action": "image", "image": "ghcr.io/myorg/web-app:v1.2.0"}' \
  https://oops.web.oops/update
```

---

## Quickstart

### 1-Line Standalone Installer: Oops CLI
Auto-detects your OS and architecture (`darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`, Google Container-Optimized OS), installs the native binary, and configures shell tab completion:
```bash
curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash
```

### Provisioning & Booting an Oopsbox Workspace
Create a brand new workstation workspace from the latest release blueprint and boot it:
```bash
# 1. Create a new workspace (downloads blueprint, generates .env credentials, sets active box)
oops box init ~/oopsbox

# 2. Boot engine, configure DNS resolver, and start default services
cd ~/oopsbox
oops box start

# 3. Install local Caddy CA root certificate into Keychain/Trust store for trusted HTTPS
oops box cert
```

### Multi-Box Switching
Switch seamlessly between isolated project workspaces:
```bash
oops box switch ~/Workspaces/client-b/oopsbox
```

### Run Oops from Anywhere (Docker Wrapper)
To run `oops` without installing Go toolchains or in locked-down environments (e.g. Google Container-Optimized OS / COS), point `OOPSBOX_DIR` to your workspace and add this alias to your shell profile (`~/.bashrc` or `~/.zshrc`):

```bash
export OOPSBOX_DIR="$HOME/oopsbox"

alias oops='docker run --rm -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "${OOPSBOX_DIR:-$HOME/oopsbox}":"${OOPSBOX_DIR:-$HOME/oopsbox}" \
  -w "${OOPSBOX_DIR:-$HOME/oopsbox}" \
  -e OOPSBOX_DIR="${OOPSBOX_DIR:-$HOME/oopsbox}" \
  ghcr.io/noyzilla/oops:latest'
```

---

## Architecture & Specifications

- [System Architecture](ARCHITECTURE.md)
- [Domain Glossary & Ubiquitous Language](CONTEXT.md)
- [Oopsbox Master Blueprint Guide](oopsbox/README.md)
- [CLI Orchestration Specification](docs/specs/cli.md)
- [Oopsbox Workstation Tooling Specification](docs/specs/oopsbox.md)
- [Native DNS Daemon Specification](docs/specs/dns.md)
- [Webhook Daemon Specification](docs/specs/webhook.md)
- [System Documentation Taxonomy](docs/README.md)
