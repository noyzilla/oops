---
title: Oops CLI Core Architecture & Routing Protocol
status: active
tags: [cli, docker, compose, rolling-update, lifecycle-hooks, database, backup]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/dns.md", "docs/specs/volume-guard.md", "docs/specs/env-sync.md", "docs/specs/remote.md"]
---

# Specification: Oops CLI Core Architecture & Routing Protocol

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/oopsbox.md](oopsbox.md), [docs/specs/dns.md](dns.md), [docs/specs/volume-guard.md](volume-guard.md), [docs/specs/env-sync.md](env-sync.md), [docs/specs/remote.md](remote.md)

## Overview & Scope

The `oops` CLI is an ergonomic orchestrator over Docker Compose. It abstracts complex multi-file chaining, service-level updates, database provisioning, automated backups, DNS discovery, and remote deployment. It enforces a strict **Multi `-f` Chaining Protocol** without relying on Compose `include` directives, isolates machine overrides via **Tail-Chaining**, and standardizes exclusively on **`.yaml`** manifests.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Group (`+<group>`)**: Logical aggregation of service templates defined in root `compose.yaml` under `x-groups:` (e.g. `+db`, `+monitoring`).
- **Project (`/<project>`)**: Dedicated application stack composition defined in `compose.<project>.yaml` (e.g. `/myapp`).
- **Service Template**: Reusable atomic service definitions located in `services/<service>.yaml` (e.g. `services/mysql.yaml`, `services/redis.yaml`).
- **Remote Overlay (`@<remote>`)**: Environment specialization capsule located in `/@<remote>/` (e.g. `/@prod/`).
- **Config**: Version-controlled service configurations residing modularly inside `config/<service>/` (e.g. `config/caddy/Caddyfile`, `config/mysql/my.cnf`).
- **Rolling Update**: Sequential pull -> stop hook -> recreate -> health poll workflow.

## Business Rules & Logic Invariants

### Smart Target Resolution & Multi `-f` Pipeline
Target strings are resolved using an explicit, shell-safe notation that eliminates quoting overhead:

| Target Syntax | Target Type | Resolution & `-f` Execution Chain |
| :--- | :--- | :--- |
| `(none)` | Base Default | `-f compose.yaml [-f compose.local.yaml] up` |
| `<service>` | Single Service | Auto-detects `services/<service>.yaml`, executing `-f compose.yaml -f services/<service>.yaml [-f compose.local.yaml] up <service>` |
| `+<group>` | Service Group | Resolves members from `x-groups.<group>` in `compose.yaml`, chaining member templates + `[-f compose.local.yaml]` |
| `/<project>` | Project Stack | Resolves dependencies (`depends_on` / `x-services`), chaining `-f compose.yaml [dependencies] -f compose.<project>.yaml [-f compose.<project>.local.yaml] [-f compose.local.yaml] up` |
| `@<remote>` | Remote Host Capsule | Resolves capsule `/@<remote>/` and deploys via Direct SSH Streaming (`oops deploy`) |
| `..wildcard..` | Wildcard Pattern | Matches service names matching prefix (`app..`), suffix (`..worker`), or substring (`..api..`). |

#### CLI Argument Bracketing & Sigil-Outside Invariant [มาตรฐานการใช้วงเล็บอาร์กิวเมนต์]
All command signatures adhere to the ubiquitous language defined in [CONTEXT.md](../../CONTEXT.md):
- **Required**: `sigil<placeholder>` (e.g. `@<remote>`, `/<project>`, `+<group>`, `<service>`).
- **Optional**: `[sigil<placeholder>]` (e.g. `[@<remote>]`, `[/<project>]`, `[+<group>]`, `[<service>]`).
- **Sigil-Outside Invariant**: Sigils (`@`, `/`, `+`) strictly stay outside `<...>` to prevent collision with HTML/XML tags (e.g. `</project>`).

### Tail-Chaining Invariant (Left-to-Right Precedence)
All machine-specific overrides (`*.local.yaml`) are strictly appended to the tail of the `-f` flag pipeline:
1. `-f compose.yaml` (Root anchor for working directory)
2. `-f services/<service>.yaml` (Atomic templates)
3. `-f compose.<project>.yaml` (Project configuration)
4. `-f compose.<project>.local.yaml` (Project-specific machine override)
5. `-f compose.local.yaml` (Global machine override — tailmost position)

### Strict `.yaml` Validation & Legacy Deprecation
- Only manifests ending with `.yaml` are recognized by discovery and execution engines.
- `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `oops.yml`, `oopsbox.yml`, and all `*.yml` variants are formally dropped.

### Workspace Directory Resolution (`ResolveWorkDir`)

To ensure multi-project stability and 100% path parity across host and containerized runtimes, the CLI resolves its target workspace directory according to the following strict hierarchy:

| Priority | Resolution Source | Condition / Invariant | Purpose & Rationale |
| :---: | :--- | :--- | :--- |
| **1** | **Explicit Flag (`-C <dir>`, `--dir <dir>`)** | Given non-empty string | Highest precedence; explicitly overrides all shell environment variables and active directory contexts. |
| **2** | **Current Directory (`.`)** | `hasComposeContent(".") == true` (contains root `compose.yaml`) | **Active Developer Context**: When inside an Oopsbox workspace, `.` takes precedence over global environment variables. |
| **3** | **Active Box State (`~/.oops/active_box`)** | Outside an Oopsbox directory and `hasComposeContent(activeBox) == true` | **Workstation Active Hub**: Automatically targets the active box registered by `oops box start` or `oops box switch`. |
| **4** | **Environment Variable (`OOPSBOX_DIR`)** | Outside an Oopsbox directory and no active box | **Explicit Environment Override**: Overrides active workspace via shell session environment variable. |
| **5** | **Standard Workstation Default (`$HOME/oopsbox`)** | `hasComposeContent("$HOME/oopsbox") == true` | Standard turnkey installation path on macOS and developer workstations. |
| **6** | **Standard Server Default (`/opt/oopsbox`)** | `hasComposeContent("/opt/oopsbox") == true` | Standard turnkey production deployment path on Linux cloud servers. |
| **7** | **Fallback (`.`)** | None of the above matched | Returns `.` where subsequent validation returns a clear error if not an Oopsbox directory. |

### Docker Wrapper Same-Host-Path Binding Invariant
When running the Oops CLI via Docker wrapper container, the host directory MUST be bound to the **identical absolute path** inside the container (`-v "${OOPSBOX_DIR}":"${OOPSBOX_DIR}" -w "${OOPSBOX_DIR}"`). Docker Compose passes working directory labels to the host daemon; same-host-path binding ensures 100% path parity.

### Sequential Lifecycle Hooks & Inter-Service Delay Protocol
When executing group lifecycle commands (`oops stop`, `oops restart`, `oops down`, or `oops up` targeting wildcards):
- **Sequential Service Execution**: Matched services are processed sequentially one by one in resolved dependency or lexicographical order.
- **Native Docker Compose Hooks**: Relies entirely on Docker Compose native lifecycle hooks (`pre_start`, `post_start`, `pre_stop`) defined within `compose.yaml`.
- **Inter-Service Delay Gap (`--delay, -d <duration|int>`)**: Introduces a pause between each consecutive service operation in a group.

### Sequential Rolling Update Algorithm (`oops update <targets...>`)
For each matched service in target order:
- **Step 1 - Pull**: Run `docker compose -f <chain> pull <service>`.
- **Step 2 - Recreate**: Run `docker compose -f <chain> up -d --no-deps <service>`.
- **Step 3 - Health Polling**:
  - Poll Docker container inspection state `.State.Health.Status` every `OOPS_HEALTHCHECK_INTERVAL` (default 3s).
  - If `.State.Health.Status == "healthy"`: Service update succeeded. If `-d` / `--delay` is set, pause for delay gap.
  - If status is `"unhealthy"` or polling exceeds `OOPS_HEALTHCHECK_TIMEOUT` (default 10m): Abort update immediately and exit Code 1.

### Database Management (`oops db <mysql|postgres>[:<target>] <action>`)
- **Supported Engines**: `mysql`, `postgres`
- **Actions**: `create`, `readonly`, `passwd`, `list`, `drop`
- **Naming Rule**: Database names contain at most one underscore (`_`), restricting databases to 2 levels.
- **Auto Credential Export**: Automatically exports credentials to `config/env/<engine>.<db_name>.env`.

### Automated Backup Suite (`oops backup`, `oops backup-db`, `oops backup-data`)
- **Unified Full Backup (`oops backup [-r <duration|int>]`)**: Runs database dumps and filesystem volume archives.
- **Database Backup (`oops backup-db [targets...] [-r <duration|int>]`)**: Dumps running database containers to `backups/<engine>_backup_<TIMESTAMP>.sql.gz`.
- **Filesystem Backup (`oops backup-data [names...] [-r <duration|int>]`)**: Archives data volumes from `oops.yaml` to `backups/data_<name>_<TIMESTAMP>.tar.gz`.

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops` | `[-C, --dir <path>]` | Displays help menu and available subcommands |
| `oops up` | `[targets...] [-d 0s]` | Starts services, groups (`+<group>`), or projects (`/<project>`) with optional delay |
| `oops stop` | `[targets...] [-x, --except <tgt>] [-d 0s]` | Gracefully stops target services |
| `oops restart`| `[targets...] [-d 0s] [-i <img/alias>]` | Restarts target services with stop hooks and delay |
| `oops down` | `[-d 0s] [--wipe-all] [-y]` | Tears down running containers |
| `oops status` | `[targets...]` | Formatted table of containers, health status, and ports |
| `oops logs` | `[targets...] [--tail 50] [-f]` | Tail service logs |
| `oops pull` | `[targets...] [--all]` | Pulls images for targets |
| `oops update` | `[targets...] [--all] [-i <img/alias>] [-d 0s]` | Executes sequential rolling update with health check |
| `oops db` | `<mysql\|postgres>[:<target>] <action>` | DB provisioning: `create`, `passwd`, `list`, `drop` |
| `oops backup` | `[-r 7d]` | Full backup (DB + volumes) and retention prune |
| `oops backup-db` | `[targets...] [-r 7d]` | DB dump and retention prune |
| `oops backup-data` | `[targets...] [-r 7d]` | Data volume tar archives and retention prune |
| `oops dns` | `[list]` | Inspects active DNS records and discovery routes |
| `oops dns add` | `<domain> <ip>` | Adds static DNS record in `config/oops/dns` |
| `oops dns del` | `<domain>` | Deletes static DNS record from `config/oops/dns` |
| `oops dns reload` | *(none)* | Atomically reloads DNS daemon |
| `oops ip` | `[service]` | Displays IP address of container |
| `oops box init` | `[path]` | Initializes Oopsbox workspace from blueprint, sets active box |
| `oops box start` | *(none)* | Boots VM engine and starts default stack |
| `oops box stop` | *(none)* | Stops all running stacks on active box |
| `oops box switch` | `<path>` | Forcefully wipes all containers, updates active box, and boots target box |
| `oops box cert` | *(none)* | Installs local Caddy CA root certificate into host OS trust store |
| `oops deploy` | `[/<stack>] [@<remote>]` | Exact ergonomic alias for `oops remote push` (pure streaming delivery) |
| `oops remote push` | `[/<stack>] [@<remote>]` | Streams manifests and capsule overlay to remote server (filtered by `.oopsallow`; pure delivery) |
| `oops remote pull` | `[/<stack>] [@<remote>]` | Downloads remote configs and manifests into local capsule `/@<remote>/` (filtered by `.oopsallow`) |
| `oops remote connect` | `@<remote>[:<path>] [path] [--path <p>]` | Connects to remote host, auto-bootstraps environment, initializes capsule with blueprints, writes `/@<host>/remote.yaml` |
| `oops remote setup` | `[@<remote>]` | Installs or upgrades `docker-compose-plugin` and `oops` CLI to latest version on remote server |
| `oops remote list` | *(none)* | Lists registered remotes discovered in `/@*/remote.yaml` |
| `oops remote remove` | `@<remote>` | Removes registered remote directory `/@<remote>/` |
| `oops rx` | `[@<remote>] <oops-cmd> [args...]` | Remote Execute: Forwards oops command over SSH to remote target (e.g. `oops rx ps` -> `oops ps`) |
| `oops version` | `[-v, --version]` | Displays Oops version and build information |
| `oops selfupdate` | `[-c, --check] [-f, --force]` | Self-updates binary from GitHub release |
| `oops server` | `[-p, --port 8080] [-c <path>]` | Starts webhook deployment and DNS discovery daemon |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: CLI human operators, CI/CD runners.
- **Downstream Dependencies**: Host Docker Engine API (`/var/run/docker.sock`), Docker Compose CLI.
- **Bounded Blast Radius**:
  - `cmd/`: CLI root and subcommands routing.
  - `internal/config/`: Configuration loader.
  - `internal/docker/`: Compose resolver, SDK client.
  - `internal/orchestrator/`: Rolling update engine.
  - `internal/sync/`: 1:1 Directory sync engine.
  - `internal/remote/`: Direct SSH streaming engine.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./...`
- **Acceptance Scenarios**:
  - `oops up +db` locates member templates declared in `x-groups.db` and chains them with `-f compose.yaml`.
  - `oops up mysql` locates `services/mysql.yaml` and executes cleanly.
  - `oops up /myapp` locates `compose.myapp.yaml` and chains dependencies.
  - Non-existent files or `.yml` files produce immediate actionable errors.
