# Oopsbox Blueprint

Oopsbox is the production-ready, turnkey Infrastructure as Code (IaC) blueprint for **Oops**. It enforces **1:1 Dev-to-Prod Parity** across local developer workstations (macOS / Linux) and production servers, while enabling fully **Git-driven infrastructure updates** with zero manual SSH configuration editing.

## Directory Topology

```text
~/oopsbox/                # Standard Root Directory (or /opt/oopsbox on production)
├── .env                  # Environment variables & secrets (copy from .env.example)
├── .env.example          # Credentials template (Committed)
├── oops.yaml             # Daemon orchestrator compose manifest & x-oops config (Committed)
├── oops.yaml.example     # Daemon orchestrator blueprint for remote /@<remote>/oops.yaml
├── oopsbox.yaml          # Local Workstation Engine & DNS settings (git-ignored)
├── oopsbox.yaml.example  # Workstation configuration template (Committed)
├── compose.yaml          # Local Root Base Manifest
├── compose.yaml.example  # Blueprint for remote /@<remote>/compose.yaml
├── .oopsallow.example    # Whitelist blueprint for capsule sync
├── services/             # Atomic service definitions (caddy, mysql, postgres, redis, etc.)
├── config/               # Version-controlled service configuration (e.g., caddy/Caddyfile)
├── data/                 # Live realtime container storage (High-IOPS persistent volume)
│   ├── mysql/            # MySQL storage
│   ├── postgres/         # PostgreSQL storage
│   ├── caddy/            # SSL certificates
│   └── oops/             # Oops Daemon runtime data
│       └── dns.records   # Local Node Custom DNS Records
└── backups/              # Secondary backup storage (Cold storage / database dumps)
```

---

## Network Segmentation & Security

Oops Stack enforces the **Principle of Least Privilege & Network Isolation**:

```mermaid
graph TD
    Internet((Internet / Clients)) -->|Ports 80/443| Edge["Edge Group (Caddy + Oops Daemon)"]
    
    subgraph "oops_edge (Ingress Only)"
        Edge -->|Reverse Proxy| WebApp["apps: web-app"]
        Edge -->|Reverse Proxy| Tools["tool: httpbin / mailpit"]
    end
    
    subgraph "oops_db (Internal & Isolated)"
        WebApp -->|SQL / Cache| DB["db: MySQL / Postgres / Redis"]
        Worker["apps: worker (Isolated / NO Internet)"] -->|Queue / DB| DB
    end
    
    style Edge fill:#3b82f6,stroke:#1d4ed8,color:#fff
    style DB fill:#10b981,stroke:#047857,color:#fff
    style Worker fill:#f59e0b,stroke:#b45309,color:#fff
```

- **`oops_edge`**: Public Ingress traffic only. Databases and background workers are **never** joined to this network.
- **`oops_db`**: Internal private network for persistence and cache. Completely isolated from the internet and Reverse Proxy.
- **Private Services (e.g. `worker`)**: Joined strictly to `oops_db`. Cannot be accessed or probed from the external web.

---

## Quickstart (Local Development)

### Create or Initialize Workspace (`oops box init [path]`)
Provision a new Oopsbox workspace from the latest official blueprint with generated cryptographic credentials and active box registration:
```bash
oops box init ~/oopsbox
```

### Start Dev Environment (`oops box start`)
Starts VM engine (OrbStack/Colima/Docker), configures local DNS resolver, syncs DNS records, and boots default stack:
```bash
cd ~/oopsbox
oops box start
```

### Install Trusted Local SSL Certificate (`oops box cert`)
Adds Caddy's local root CA certificate to macOS Keychain or Linux system trust store (enables green locks for `https://*.web.oops`):
```bash
oops box cert
```

### Switch Active Oopsbox (`oops box switch <path>`)
Seamlessly switch between multiple isolated organization workspaces without port or container name collisions:
```bash
oops box switch ~/Workspaces/another-org/oopsbox
```

---

## Unified Configuration (`oops.yaml`)

`oops.yaml` is a Docker Compose manifest holding the `oops` daemon service alongside orchestrator settings under `x-oops:` (registries, backups, DNS):

```yaml
services:
  oops:
    image: ghcr.io/noyzilla/oops:edge
    restart: unless-stopped
    networks:
      - oops_edge
    # ...

x-oops:
  registries:
    gar: asia-southeast1-docker.pkg.dev/my-project/my-repo
    my_org: ghcr.io/my_org
    hub: docker.io/myorg

  dns:
    upstreams:
      - 1.1.1.1:53
      - 8.8.8.8:53
    records:
      # - api.internal 10.0.0.5
      # - .staging.oops 10.0.0.10
```

Use the `/<stack>` target syntax with any `oops` command:
```bash
oops up                     # Starts the root base stack and its dependencies
oops up /edge-caddy         # Starts Edge Perimeter only
oops up /db                 # Starts Databases
oops up /storage            # Starts Object Storage (SeaweedFS)
oops restart /edge-caddy

# Switch active stack (starts target stack & stops all other running services):
oops switch /apps

# Stop all services except specified exclusions:
oops stop -x /edge-caddy
oops stop -x /edge-caddy -x redis
```

---

## Compose Extension Fields

Oops manifests use standardized extension keys for service resolution and lifecycle grouping:

| Extension Field | Placement | Purpose & Behavior |
| :--- | :--- | :--- |
| **`x-oops-groups:`** | `compose.yaml` | Groups services and stacks for batch targeting (`+<group>`), e.g. `oops up +db` |
| **`x-oops-depends:`** | `compose.<stack>.yaml` | Runtime dependencies auto-started along with target stack (`oops up /<stack>`) |
| **`x-oops-services:`** | `compose.<stack>.yaml` | Bundles service templates into the compose chain without auto-starting |

---

## DNS Management CLI (`oops dns`)

Inspect, query, and manage DNS resolution directly:
```bash
# List all active DNS records (Custom, Infra, and Container Services)
oops dns
oops dns list

# Query resolved IP for a domain
oops dns get host.oops
oops dns get mysql.oops

# Set or update a custom record (upsert)
oops dns set my-app.test 127.0.0.1
oops dns set .wild.test 192.168.1.100

# Delete a custom record
oops dns del my-app.test

# Validate and reload daemon DNS table
oops dns reload
```

---

## Global Oops CLI Usage

You can also use `oops` natively or via container alias from any directory:

```bash
# Set up Host Alias in ~/.zshrc or ~/.bashrc:
alias oops='docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$HOME/oopsbox":"$HOME/oopsbox" \
  -w "$HOME/oopsbox" \
  ghcr.io/noyzilla/oops:latest'

# Pull images across all stacks, specific stack, or registry alias
oops pull --all
oops pull /apps
oops pull gar/my-app:v1.0.0

# Start service groups or stacks
oops up                     # Starts default root stack (/. mapping to compose.yaml)
oops up /edge-caddy         # Starts Edge Perimeter (Caddy + Oops DNS)
oops up /db                 # Starts Databases (MySQL, Postgres, Redis)
oops up /storage            # Starts Object Storage (SeaweedFS)
oops up /tool               # Starts Dev Tools (httpbin, mailpit)
oops up /apps               # Starts Applications

# Target specific services or wildcard
oops up mysql
oops up /db/mysql
oops up httpbin
oops up mailpit
oops up app..

# Switch profiles and stop other services
oops switch /edge-caddy
oops stop -x /edge-caddy
```

---

## Stack Operations

### Sequential Rolling Updates (Zero Downtime)
Deploy updates across apps sequentially with automated health check polling:
```bash
oops update app.. -d 2s
oops update app1
oops update --image gar/my-app:v1.0.0
```

### Database Provisioning & Password Management
Create databases, dedicated users, and rotate passwords (auto-generates 20-character secure passwords and exports credentials to `config/env/<container>.<db>.env`):
```bash
# MySQL
oops db mysql create myapp_db                       # Auto-generates password & exports .env (User implicitly matches DB name)
oops db mysql:mysql-analytics create report_db      # Target specific container (exports config/env/mysql-analytics.report_db.env)
oops db mysql readonly myapp_db metabase            # Provision SELECT-only readonly user (myapp_db__ro_metabase)
oops db mysql passwd myapp_db                       # Rotate password (auto-generates new)
oops db mysql passwd myapp_db myNewPass123          # Set explicit password
oops db mysql create app_db -n                      # Skip exporting .env file
oops db mysql list
oops db mysql drop myapp_db

# PostgreSQL
oops db postgres create myapp_db                    # Auto-generates password
oops db postgres:pg-custom create analytics_db      # Target specific container
oops db postgres readonly myapp_db metabase         # Provision SELECT-only readonly user
oops db postgres passwd myapp_db                    # Rotate password (auto-generates new)
oops db postgres list
oops db postgres drop myapp_db
```

### Automated Backup Suite (Database & Data Volumes)
Execute automated database dumps, filesystem data packaging, and prune expired archives older than `retention` (default `7d`):
```bash
# Full System Backup (Database Dumps + Data Volumes)
oops backup                         # Full backup (DB + Data) and prune expired
oops backup prune                   # Prune all expired archives (no dump)

# Database Backup Only (MySQL, PostgreSQL)
oops backup-db                      # Backup all databases
oops backup-db mysql                # Backup specific database
oops backup-db postgres -r 14d      # Backup with custom 14-day retention
oops backup-db prune                # Prune expired DB dump archives

# Data Volumes & Filesystem Backup Only (Uploads, Storage, Certs)
oops backup-data                    # Backup all targets defined in oops.yaml
oops backup-data uploads            # Backup specific data target
oops backup-data prune              # Prune expired data volume archives
```

### Webhook Automation (CI/CD)
Trigger deployments from GitHub Actions or GitLab CI via the Webhook Engine:
```bash
curl -X POST https://webhook.yourdomain.com/deploy \
  -H "Authorization: Bearer your_secret_token" \
  -H "Content-Type: application/json" \
  -d '{"target": "app..", "mode": "image"}'
```
