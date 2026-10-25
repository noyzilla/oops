---
title: Oops CLI Core Architecture & Routing Protocol
status: active
tags: [cli, docker, compose, rolling-update, lifecycle-hooks, database, backup]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/dns.md", "docs/specs/volume-guard.md", "docs/specs/remote.md"]
---

# Specification: Oops CLI Core Architecture & Routing Protocol

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/oopsbox.md](oopsbox.md), [docs/specs/dns.md](dns.md), [docs/specs/volume-guard.md](volume-guard.md), [docs/specs/remote.md](remote.md)

## Overview & Scope

The `oops` CLI is an ergonomic orchestrator over Docker Compose. It abstracts complex multi-file chaining, service-level updates, database provisioning, automated backups, DNS discovery, and remote deployment. It enforces a strict **Multi `-f` Chaining Protocol** without relying on Compose `include` directives, isolates machine overrides via **Tail-Chaining**, and standardizes exclusively on **`.yaml`** manifests.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Stack (`/<stack>`)**: Dedicated infrastructure or application stack composition defined in `compose.<stack>.yaml` (e.g. `/edge`, `/db`, `/tools`, `/myapp`).
- **Stack Order & Auto-Start (`x-oops-stack`)**: Ordered list declared in `compose.yaml` (or inside `compose.<stack>.yaml`) defining which stacks or services automatically start on `oops up` and their exact top-to-bottom boot sequence.
- **Scoped Service (`/<stack>/<service>` or `<service>`)**: Specific container service inside a stack (e.g. `/db/mysql` or `mysql` resolved from `compose.db.yaml`).
- **Remote Overlay (`@<remote>`)**: Environment specialization capsule located in `/@<remote>/` (e.g. `/@prod/`).
- **Config**: Version-controlled service configurations residing modularly inside `config/<service>/` (e.g. `config/caddy/Caddyfile`, `config/mysql/my.cnf`).
- **Rolling Update**: Sequential pull -> stop hook -> recreate -> health poll workflow.

## Business Rules & Logic Invariants

### Smart Target Resolution & Multi `-f` Pipeline
Target strings are resolved using an explicit, shell-safe notation that eliminates quoting overhead:

| Target Syntax | Target Type | Resolution & `-f` Execution Chain |
| :--- | :--- | :--- |
| `(none)` | Base Default (`x-oops-stack`) | Resolves ordered entries from `x-oops-stack:` in `compose.yaml`, chaining `-f compose.yaml -f compose.<stack>.yaml [-f compose.<stack>.local.yaml] [-f compose.local.yaml] up` for each entry in sequence |
| `/<stack>` | Infrastructure / Application Stack | Resolves `x-oops-stack:` (if declared in `compose.<stack>.yaml`) and dependencies (`depends_on` / `x-oops-depends`), chaining `-f compose.yaml [dependencies] -f compose.<stack>.yaml [-f compose.<stack>.local.yaml] [-f compose.local.yaml] up` |
| `/<stack>/<service>` | Scoped Stack Service | Targets a specific service within `compose.<stack>.yaml`, resolving dependencies within the stack |
| `<service>` | Bare Service Name | Searches `compose.<stack>.yaml` files (prioritizing `db`, `edge`, `storage`, `tools`), executing `-f compose.yaml -f compose.<stack>.yaml [-f compose.<stack>.local.yaml] [-f compose.local.yaml] up <service>` |
| `@<remote>` | Remote Host Capsule | Resolves capsule `/@<remote>/` and deploys via Direct SSH Streaming (`oops deploy`) |
| `..wildcard..` | Wildcard Pattern | Matches service names matching prefix (`app..`), suffix (`..worker`), or substring (`..api..`). |

### `x-oops-stack` Ordering & Selective Auto-Start Invariant
Both root `compose.yaml` and individual `compose.<stack>.yaml` files support `x-oops-stack:`:
- **Explicit Auto-Start Whitelist**: Only stacks or services listed in `x-oops-stack:` are started automatically when running `oops up` (for `compose.yaml`) or `oops up /<stack>` (for `compose.<stack>.yaml`). Services defined in `services:` but omitted from `x-oops-stack:` remain dormant until explicitly targeted (e.g. `oops up /storage/seaweedfs`).
- **Deterministic Top-to-Bottom Execution Order**: Entries in `x-oops-stack:` are resolved and started strictly in top-to-bottom array order (e.g. `/edge` before `/db`, or `mysql` before `app`).
- **Recursive Resolution**: When `/<stack>` is referenced in `compose.yaml` `x-oops-stack:`, the resolver inspects `compose.<stack>.yaml` and honors its internal `x-oops-stack:` list if present.

### Local Service Precedence Invariant
When resolving dependencies (`depends_on`, `x-oops-depends`, `x-oops-services`) within a stack (`compose.<stack>.yaml`), any dependency whose name matches a service defined locally within that same stack takes absolute precedence:
- The local service definition is used directly within the stack.
- Cross-stack resolution is only triggered if no local service definition with the matching name exists in the active stack.
- CLI target `/<stack>/<service>` targets the specific service defined within that stack, while bare `<service>` searches across workspace stack files.

#### CLI Argument Bracketing & Sigil-Outside Invariant [มาตรฐานการใช้วงเล็บอาร์กิวเมนต์]
All command signatures adhere to the ubiquitous language defined in [CONTEXT.md](../../CONTEXT.md):
- **Required**: `sigil<placeholder>` (e.g. `@<remote>`, `/<stack>`, `/<stack>/<service>`, `<service>`).
- **Optional**: `[sigil<placeholder>]` (e.g. `[@<remote>]`, `[/<stack>]`, `[<service>]`).
- **Sigil-Outside Invariant**: Sigils (`@`, `/`) strictly stay outside `<...>` to prevent collision with HTML/XML tags (e.g. `</stack>`).

### Tail-Chaining Invariant (Left-to-Right Precedence)
All machine-specific overrides (`*.local.yaml`) are strictly appended to the tail of the `-f` flag pipeline:
- `-f compose.yaml` (Root anchor for working directory and `oops_default` network)
- `-f compose.<dep-stack>.yaml` (Cross-stack dependencies if applicable)
- `-f compose.<stack>.yaml` (Target stack configuration and stack-scoped networks)
- `-f compose.<stack>.local.yaml` (Stack-specific machine override)
- `-f compose.local.yaml` (Global machine override — tailmost position)
- **Orphan Warning Suppression (`COMPOSE_IGNORE_ORPHANS=True`)**: Because all stacks share the root `compose.yaml` project namespace (`oopsbox`), `docker compose` invocations automatically inject `COMPOSE_IGNORE_ORPHANS=True` into the process environment so running containers from sibling stacks are never flagged as orphans.
- Note: `oops.yaml` is a pure Oops CLI/daemon configuration file (`x-oops:`) and is never passed via `-f` to `docker compose`.

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

### Docker Daemon & Context Host Resolution Invariant
All interactions with the local Docker daemon via the Go Docker SDK are centralized through `internal/docker.NewClient()`. The client dynamically resolves the target Docker daemon endpoint following this strict precedence:
- **Environment Override (`$DOCKER_HOST`)**: If `$DOCKER_HOST` is explicitly set in the execution environment, its value takes absolute precedence.
- **Active Docker Context Inspection**: If `$DOCKER_HOST` is unset, Oops queries the active Docker context endpoint by executing `docker context inspect --format '{{.Endpoints.docker.Host}}'`. This guarantees full parity with Docker CLI context selections (such as OrbStack `unix://$HOME/.orbstack/run/docker.sock`, Colima, or custom remote contexts) without requiring manual shell environment exports.
- **Default Daemon Fallback**: If context inspection fails, returns empty, or Docker CLI is not installed, the client falls back to the default platform socket (`client.DefaultDockerHost` / `unix:///var/run/docker.sock`).

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
  - Poll Docker container inspection state `.State.Health.Status` every `OOPS_HEALTHCHECK_INTERVAL` (default 3s, loaded from `x-oops.orchestrator.healthcheck.interval` with env override).
  - If `.State.Health.Status == "healthy"`: Service update succeeded. If `-d` / `--delay` is set, pause for delay gap.
  - If status is `"unhealthy"` or polling exceeds `OOPS_HEALTHCHECK_TIMEOUT` (default 10m, loaded from `x-oops.orchestrator.healthcheck.timeout` with env override): Abort update immediately and exit Code 1.

### Database Management (`oops db <mysql|postgres|redis>[:<target>] <action>`)
- **Supported Engines**: `mysql`, `postgres`, `redis`
- **Actions**: `create`, `readonly`, `passwd`, `list`, `drop`
- **Naming Rule & Terminology**: Relational database names (`mysql`, `postgres`) contain at most one underscore (`_`), restricting databases to 2 levels. For Redis, this serves as the Key Prefix namespace (`~<prefix>:*`) without underscore count restriction; output labels and CLI usage messages dynamically use `Prefix` and `<prefix>` instead of `Database` and `<db_name>`.
- **Redis ACL Permissions**: Redis ACL users are granted `+@all -@dangerous +info` (`+@read +info` for readonly), permitting standard client library health and ready checks via `INFO` while blocking blocking operations like `KEYS` in favor of cursor-based `SCAN`.
- **Auto Credential Export**: Automatically exports pure engine-scoped credentials to `config/env/<engine>.<db_name>.env` without generic `DB_*` variable collisions:
  - MySQL: `DB_ENGINE=mysql`, `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_URL`
  - PostgreSQL: `DB_ENGINE=postgres`, `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DATABASE`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_URL`
  - Redis: `DB_ENGINE=redis`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_USER`, `REDIS_PASSWORD`, `REDIS_DB=0`, `REDIS_PREFIX`, `REDIS_URL`

### Automated Backup Suite (`oops backup`, `oops backup-db`, `oops backup-data`)
- **Unified Full Backup (`oops backup [-r <duration|int>]`)**: Runs database dumps and filesystem volume archives.
- **Database Backup (`oops backup-db [targets...] [-r <duration|int>]`)**: Dumps running database containers to `backups/<engine>_backup_<TIMESTAMP>.sql.gz`.
- **Filesystem Backup (`oops backup-data [names...] [-r <duration|int>]`)**: Archives data volumes from `oops.yaml` to `backups/data_<name>_<TIMESTAMP>.tar.gz`.

### Restore Suite (`oops restore`, `oops restore-db`, `oops restore-data`)
- **Smart Restore Dispatcher (`oops restore [target_or_file] [backup_file] [-y]`)**: Directly restores databases or volumes. Supports engine shortcuts (`oops restore redis`), raw dump files (`oops restore dump.rdb`, `*.rdb`, `*.sql`, `*.tar.gz`), and automatically transpiles Redis RDB snapshots to active Redis 7 AOF structures.
- **Database Restore (`oops restore-db <engine>[:target] [db_name] [backup_file] [-y]`)**: Restores database dump into running container. For Redis, allows omitting `db_name` (`oops restore-db redis [dump.rdb]`).
- **Data Volume Restore (`oops restore-data <target_name|backup_file> [-n] [-y]`)**: Extracts compressed filesystem volume archive with dry-run preview.

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops` | `[-C, --dir <path>]` | Displays help menu and available subcommands |
| `oops up` | `[targets...] [-d 0s]` | Starts stacks (`/<stack>`), scoped services (`/<stack>/<service>`), or bare services with optional delay |
| `oops stop` | `[targets...] [-x, --except <tgt>] [-d 0s]` | Gracefully stops target services |
| `oops restart`| `[targets...] [-d 0s] [-i <img/alias>]` | Restarts target services with stop hooks and delay |
| `oops down` | `[-d 0s] [--wipe-all] [-y]` | Tears down running containers |
| `oops ps` | `[targets...] [-a, --all] [--full, -fl]` | Formatted table of `CONTAINER ID`, `STACK`, `SERVICE`, `IMAGE` (shortened by default, full path with `--full` / `-fl`), `STATUS` (with uptime & health; uncreated services shown as `missing` with `-a`), `IP`, and `PORTS` |
| `oops logs` | `[targets...] [--tail 50] [-f]` | Tail service logs |
| `oops pull` | `[targets...] [--all]` | Pulls images for targets |
| `oops update` | `[targets...] [--all] [-i <img/alias>] [-d 0s]` | Executes sequential rolling update with health check |
| `oops db` | `<mysql\|postgres\|redis>[:<target>] <action>` | DB provisioning: `create`, `readonly`, `passwd`, `list`, `drop` |
| `oops backup` | `[-r 7d]` | Full backup (DB + volumes) and retention prune |
| `oops backup-db` | `[targets...] [-r 7d]` | DB dump and retention prune |
| `oops backup-data` | `[targets...] [-r 7d]` | Data volume tar archives and retention prune |
| `oops restore` | `[target_or_file] [backup_file] [-y]` | Smart restore for engines (`redis`), dump files (`dump.rdb`), or archives |
| `oops restore-db` | `<engine>[:target] [db_name] [backup_file] [-y]` | DB dump restore with confirmation guard |
| `oops restore-data` | `<target_name\|backup_file> [-n] [-y]` | Data volume tar archive restore with dry-run preview |
| `oops dns` | `[list]` | Inspects active DNS records and discovery routes |
| `oops dns get` | `<domain>` | Resolves and outputs IP address for domain |
| `oops dns set` | `<domain> <ip>` | Adds or updates static DNS record in `data/oops/dns.records` |
| `oops dns del` | `<domain>` | Deletes static DNS record from `data/oops/dns.records` |
| `oops dns reload` | *(none)* | Atomically reloads DNS daemon |
| `oops dns prune` | `[-n, --dry-run]` | Prunes stale macOS DNS resolvers in `/etc/resolver/` (alias `oops dns fix`) |
| `oops doctor` | *(none)* | Full system and workspace health diagnostic across Compose, DNS, and Docker |
| `oops doctor compose` | `[@<remote>] [-a, --all]` | Audits Compose subnet containment, IPAM compliance, and static IP reservations |
| `oops doctor dns` | *(none)* | Audits macOS `/etc/resolver/` entries and Single-TLD configuration |
| `oops ip` | `[service]` | Displays IP address of container |
| `oops box init` | `[path]` | Initializes Oopsbox workspace from blueprint, sets active box |
| `oops box start` | *(none)* | Boots VM engine and starts default stack |
| `oops box stop` | *(none)* | Stops all running stacks on active box |
| `oops box switch` | `<path>` | Forcefully wipes all containers, updates active box, and boots target box |
| `oops box cert` | *(none)* | Installs local Caddy CA root certificate into host OS trust store |
| `oops deploy` | `[/<stack1>,/<stack2>] [@<remote>]` | Exact ergonomic alias for `oops remote push` (pure streaming delivery) |
| `oops remote push` | `[/<stack1>,/<stack2>] [@<remote>]` | Streams manifests and capsule overlay to remote server (filtered by `.oopsallow`; pure delivery) |
| `oops remote pull` | `[/<stack1>,/<stack2>] [@<remote>]` | Downloads remote configs and manifests into local capsule `/@<remote>/` (filtered by `.oopsallow`) |
| `oops remote connect` | `@<remote>[:<path>] [path] [--path <p>]` | Connects to remote host, auto-bootstraps environment, initializes capsule with blueprints, writes `/@<host>/remote.yaml` |
| `oops remote setup` | `[@<remote>]` | Installs or upgrades `docker-compose-plugin` and `oops` CLI to latest version on remote server |
| `oops remote list` | *(none)* | Lists registered remotes discovered in `/@*/remote.yaml` |
| `oops remote remove` | `@<remote>` | Removes registered remote directory `/@<remote>/` |
| `oops tunnel` | `[@<remote>] [service[:port]...]` | Establishes or inspects database tunnel to remote server (auto-detects ports and uses `tunnel.services` from `remote.yaml`) |
| `oops tunnel up` | `@<remote> [service[:port]...]` | Starts client tunnel container on Oops network without host port mappings |
| `oops tunnel down` | `@<remote>` | Stops and removes client tunnel container for target remote |
| `oops tunnel list` | *(none)* | Lists active tunnel connections, container names, and DNS endpoints (`<service>.<remote>.tunnel.oops:<port>`) |
| `oops tunnel restart` | `@<remote> [service[:port]...]` | Re-resolves remote container IPs/ports and restarts client tunnel container |
| `oops rx` | `[@<remote>] <oops-cmd> [args...]` | Remote Execute: Forwards oops command over SSH to remote target (e.g. `oops ps` -> `oops ps`) |
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
  - `oops up` reads `x-oops-stack` from `compose.yaml` and starts listed stacks/services in exact top-to-bottom order.
  - `oops up /db` locates `compose.db.yaml` and starts only services listed in `x-oops-stack` (or all services if `x-oops-stack` is omitted).
  - `oops up /db/mysql` or `oops up mysql` resolves `mysql` inside `compose.db.yaml` and starts that specific service.
  - `oops up /myapp` locates `compose.myapp.yaml` and chains cross-stack dependencies.
  - Non-existent files or `.yml` files produce immediate actionable errors.
