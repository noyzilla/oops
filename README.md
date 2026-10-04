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
├── oops.yml              # Master Config: profiles (@default, @lab), registry shortcuts, backups, DNS
├── devoops.yml           # Workstation Settings: VM engine (OrbStack/Colima), resources, local DNS
├── stacks/               # Pure Git-Tracked Infrastructure as Code (IaC)
│   ├── edge/             # Group: Ingress Reverse Proxy (Caddy) + Oops Daemon (net-edge)
│   │   ├── compose.yml
│   │   └── caddy/Caddyfile
│   ├── db/               # Group: Persistence & Cache (MySQL, Postgres, Redis on isolated net-db)
│   │   ├── compose.yml
│   │   └── mysql/my.cnf
│   ├── tool/             # Group: Dev & Mock Utilities (httpbin, mailpit on net-edge)
│   │   └── compose.yml
│   └── apps/             # Group: Application Services (web-app, worker)
│       └── compose.yml
├── bin/                  # Workstation Developer Tools (devoops, devoops-mac, devoops-linux)
├── data/                 # Live container storage (High-IOPS persistent host volumes)
└── backups/              # Automated database dumps & filesystem data archives
```

### Why Oopsbox?

- **1-Command Workstation Setup**: Run `./bin/devoops install` and `devoops start` to boot the VM, set up host networking, register the macOS/Linux DNS resolver, and trust root SSL certificates.
- **Automated Local HTTPS**: Caddy Edge Proxy automatically issues and serves valid TLS certificates with green locks for `https://*.web.oops`.
- **Embedded Zero-Config DNS**: Containers are instantly resolvable by hostname (e.g. `mysql.oops`, `redis.oops`, `host.oops`, `vm.oops`) through the embedded Oops DNS engine.
- **Strict Network Isolation**: Enforces least privilege across `net-edge` (public ingress) and `net-db` (isolated backend).
- **Single Source of Truth (`oops.yml`)**: Configure project profiles (`profiles:`), image registry aliases (`registries:`), and automated backups in one clean YAML file.

[Read the Full Oopsbox Blueprint Guide](oopsbox/README.md)

---

## The Engine: Oops CLI & Daemon

The `oops` CLI binary manages multi-group containers, performs safe rolling updates, automates database provisioning, and executes backups.

### Smart Target & Profile Selectors
```bash
# Start default profile (@default defined in oops.yml)
oops up

# Start custom project profile or whole group
oops up @lab
oops up /edge
oops up /db

# Switch active profile (starts target profile & gracefully stops others)
oops switch @lab

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
# Generate 20-char secure passwords and provision DB + User + Grants
oops db mysql create my_database my_user
oops db pg:pg-replica create analytics_db analyst_user
oops db mysql passwd my_user new_password
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

### Native DNS Inspection & Management
```bash
# Inspect active DNS resolution table (Containers + Custom Records)
oops dns
oops dns get mysql.oops

# Manage custom static / wildcard DNS records
oops dns set api.internal 10.0.0.5
oops dns set .staging.oops 127.0.0.1
oops dns del api.internal
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
| `oops.stop.cmd` | No | Command to execute inside container *before* stopping (e.g. `php artisan horizon:terminate`). |
| `oops.stop.timeout` | No | Timeout for graceful pre-stop hook (e.g. `30s`, `60s`). Default: `30s`. |
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

### Option A: Use Oopsbox (Recommended)
```bash
git clone https://github.com/noyzilla/oops.git
cd oops/oopsbox

# Initialize workstation environment (.env, storage, DNS resolver, global PATH)
./bin/devoops install

# Start environment
devoops start

# Install local root SSL certificate for green lock HTTPS (*.web.oops)
devoops install-cert
```

### Option B: Run Standalone CLI / Daemon
```bash
# Build oops binary locally
go build -o oops .

# Run webhook & DNS server
./oops server -p 80
```

---

## Architecture & Specifications

- [System Architecture](ARCHITECTURE.md)
- [Domain Glossary & Ubiquitous Language](CONTEXT.md)
- [Oopsbox Master Blueprint Guide](oopsbox/README.md)
- [CLI Orchestration Specification](docs/specs/cli.md)
- [DevOops Workstation Tooling Specification](docs/specs/devoops.md)
- [Native DNS Daemon Specification](docs/specs/dns.md)
- [Webhook Daemon Specification](docs/specs/webhook.md)
- [System Documentation Taxonomy](docs/README.md)
