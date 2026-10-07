---
title: CLI Orchestration & Stack Management
status: active
tags: [cli, docker, compose, rolling-update, lifecycle-hooks, database, backup]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "DESIGN.md", "docs/specs/webhook.md", "docs/specs/storage-guard.md", "docs/specs/deploy-key.md"]
---

# Specification: CLI Orchestration & Stack Management

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Spec**: [docs/specs/webhook.md](webhook.md), [docs/specs/storage-guard.md](storage-guard.md), [docs/specs/deploy-key.md](deploy-key.md)

## Overview & Scope

Oops provides an operator CLI binary (`oops <command>`) designed to manage multi-group Docker Compose environments (`stacks/edge`, `stacks/db`, `stacks/tool`, `stacks/apps`), execute sequential rolling updates with health check polling, enforce graceful lifecycle shutdown hooks, provision least-privilege database credentials, and manage automated database backups.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Group**: A distinct functional category and compose setup residing in `stacks/<group>/` containing a dedicated `compose.yml` (e.g. `/edge`, `/db`, `/tool`, `/apps`, or custom sub-directories).
- **Service Configuration**: Version-controlled service configuration files residing modularly inside each group (`stacks/<group>/<service>/`, e.g. `stacks/edge/caddy/Caddyfile`, `stacks/db/mysql/my.cnf`).
- **Profile**: A named workstation or project group defined in `oops.yml` (`profiles:`), selectable via `@<profile>` (e.g. `@default`, `@lab`).
- **Target**: Explicit group (`/db`, `/apps`), scoped service (`/db/mysql`, `/apps/api`), exact service name (`caddy`, `mysql`), or Double Dot wildcard (`app..`, `..worker`, `..api..`).
- **Rolling Update**: Sequential pull -> stop hook -> recreate -> health poll workflow.

## Business Rules & Logic Invariants

### Smart Target Resolution & Double Dot (`..`) Wildcard Protocol
Target strings are resolved using an explicit, shell-safe notation that eliminates quoting overhead on physical and virtual keyboards:
- **Profile Stacks (`@<profile>`)**:
  - Any target starting with `@` (e.g., `@default`, `@lab`) resolves against `profiles:` in `oops.yml`. It expands recursively into the constituent stacks and services defined in the named profile.
  - **Dynamic `@all` Selector**: `@all` is a native dynamic selector that automatically discovers and expands all stacks and services across the entire workspace in dependency order without requiring manual maintenance in `oops.yml`.
- **Registry Aliases & Image Matching (`<alias>/<image>:<tag>` or `<image>:<tag>`)**:
  - Any target matching a configured registry alias (e.g. `gar/web-app:v1.0` -> `asia-southeast1-docker.pkg.dev/.../web-app:v1.0`), `img:<image>`, or exact image name resolves to all services across all stacks using that container image.
- **Stack Target (`/<stack>`)**:
  - Any target starting with a leading slash `/` without further subpaths (e.g., `/edge`, `/db`, `/apps`) targets the entire stack and executes on that stack's `compose.yml` (located under `stacks/<stack>/`).
- **Scoped Service Target (`/<stack>/<service>`)**:
  - Path notation (e.g., `/db/mysql`, `/apps/web`) targets only the specified service strictly inside the designated stack.
- **Exact Service Name (Bare string without `..`)**:
  - A bare string without `..` (e.g., `mysql`, `caddy`) strictly matches only an exact service name. If no exact match exists, an error is returned.
- **Double Dot (`..`) Wildcard Matcher**:
  - **Prefix Match (`<prefix>..`)**: e.g., `app..` matches all services starting with `app` (e.g. `app-web`, `app-worker`).
  - **Suffix Match (`..<suffix>`)**: e.g., `..worker` matches all services ending with `worker` (e.g. `mail-worker`, `job-worker`).
  - **Contains Match (`..<keyword>..`)**: e.g., `..api..` matches any service name containing `api`.
- **Default Profile & Global Stack (Omitted Target)**:
  - If no target is specified, the CLI defaults to `@default` if defined in `oops.yml` (`profiles.default`).
  - If `@default` is not defined, operations execute across all stacks in deterministic dependency order:
    - **Startup (`oops up`)**: `/edge` -> `/db` -> `/tool` -> `/apps` -> `[custom stacks...]`
    - **Teardown (`oops down`)**: `[custom stacks...]` -> `/apps` -> `/tool` -> `/db` -> `/edge`

### Workspace Directory Resolution (`ResolveWorkDir`)

To ensure seamless multi-organization operations, eliminate accidental cross-environment execution, and guarantee 100% path parity across host and containerized runtimes, the Oops CLI resolves its target workspace directory (`workDir`) according to the following strict hierarchy:

| Priority | Resolution Source | Condition / Invariant | Purpose & Rationale |
| :---: | :--- | :--- | :--- |
| **1** | **Explicit Flag (`-C <dir>`, `--dir <dir>`)** | Given non-empty string | Highest precedence; explicitly overrides all shell environment variables and active directory contexts. |
| **2** | **Current Directory (`.`)** | `hasComposeContent(".") == true` (contains `stacks/` directory or root `compose.yml`) | **Active Developer Context**: If a developer intentionally `cd`s into an Oopsbox workspace (e.g. `~/Workspaces/org-a/oopsbox`), `.` takes precedence over global environment variables (`OOPSBOX_DIR`) to prevent accidental operations on the wrong project. |
| **3** | **Active Box State (`~/.oops/active_box`)** | Outside an Oopsbox directory and `hasComposeContent(activeBox) == true` | **Workstation Active Hub**: Automatically targets the active box registered by `oopsbox start`, `oopsbox switch`, or `oopsbox install` without requiring manual env vars. |
| **4** | **Environment Variable (`OOPSBOX_DIR`)** | Outside an Oopsbox directory and no active box | **Explicit Environment Override**: Allows overriding active workspace via shell session environment variable. |
| **5** | **Standard Workstation Default (`$HOME/oopsbox`)** | `hasComposeContent("$HOME/oopsbox") == true` | Standard turnkey installation path on macOS and developer workstations. |
| **6** | **Standard Server Default (`/opt/oopsbox`)** | `hasComposeContent("/opt/oopsbox") == true` | Standard turnkey production deployment path on Linux cloud servers. |
| **7** | **Fallback (`.`)** | None of the above matched | Returns `.` where subsequent validation (`DiscoverStacks`) returns a clear error if not an Oopsbox directory. |

#### Active Box Verification & Mismatch Guard
To prevent accidental cross-box collisions when multiple Oopsbox repositories exist on the same workstation:
- When executing lifecycle mutation commands (`up`, `restart`, `update`, `switch`, `pull`, `db`, `dns`), `ValidateActiveBox` verifies that the resolved target workspace matches `~/.oops/active_box`.
- If a mismatch is detected (e.g., active box is `~/Workspaces/box1` while executing inside `~/Workspaces/box2`), execution aborts immediately with an actionable error directing the developer to run `oopsbox switch <path>`.
- If `~/.oops/active_box` does not exist (single-box workstation or production server), validation passes transparently.

#### Two-Stage Workspace Validation Invariants
1. **Pre-Flight Inspection (`hasComposeContent`)**: Validates that candidate directories are accessible and contain either a `stacks/` subdirectory or compose manifests (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`).
2. **Execution Guard (`DiscoverStacks`)**: When executing lifecycle commands (`up`, `stop`, `restart`, `update`, `db`, `backup`), validates that at least one valid compose stack is discovered. If zero stacks exist, execution halts immediately with:
   `Error: no compose stacks or services found in workspace "<workDir>"`.

#### Docker Wrapper Same-Host-Path Binding Invariant
When running the Oops CLI via Docker wrapper container (`ghcr.io/noyzilla/oops:latest`), the host directory MUST be bound to the **identical absolute path** inside the container (`-v "${OOPSBOX_DIR}":"${OOPSBOX_DIR}" -w "${OOPSBOX_DIR}"`):
- **Why `/workspace` binding is forbidden**: Docker Compose passes working directory labels (`com.docker.compose.project.working_dir`) and relative volume bind mounts (`./caddy/Caddyfile`) to the host Docker daemon. If bound to `/workspace`, the host daemon attempts to locate `/workspace` on the host, causing path mismatches and volume failures. Same-host-path binding ensures 100% path parity with direct host execution.

### Dual Working Modes & 3-Tier Command Context Architecture

The CLI is engineered around two distinct developer working modes:
1. **Oopsbox Infrastructure Mode**: The developer is sitting inside an **Oopsbox Workspace** (`stacks/` directory or `compose.yaml` present). Full access to remote server registration, stack editing, and server deployment (`oops deploy`).
2. **Application Developer Mode**: The developer is sitting inside an **Application Project Directory** (e.g. `~/Workspaces/my-web-app`). Full access to developer stack control & status (`oops up db`, `oops status`, `oops logs`) using the machine's **Active Box** (`~/.oops/active_box`). Attempting to deploy or register remotes from an app directory is strictly blocked to prevent code corruption on remote servers.

#### 3-Tier Command Context Matrix

| Tier | Category | Subcommands | Context Resolution & Safety Invariant |
| :--- | :--- | :--- | :--- |
| **Tier 1** | **Workspace & Git Deployment** | `oops remote`, `oops deploy`, `oops rx` | **Git Project Boundary**: Requires execution inside a Git Project repository (`.git` at root or parent) and valid Oopsbox Workspace. |
| **Tier 2** | **Local Container Orchestration** | `oops up`, `oops down`, `oops status`, `oops logs`, `oops db`, `oops backup`, `oops dns` | **Zero Git Dependency**: Operates on local containers and active box DNS/environment configuration without `.git` (standalone `oopsbox` or machine Active Box). |
| **Tier 3** | **Global Machine Tools** | `oops box`, `oops ip`, `oops version`, `oops selfupdate` | **Global Context**: Standalone machine tools runnable from any directory without restriction. |

### Sequential Lifecycle Hooks & Inter-Service Delay Protocol
When executing group lifecycle commands (`oops stop`, `oops restart`, `oops down`, or `oops up` targeting wildcards such as `app..` or whole stacks):
- **Sequential Service Execution**: Matched services are processed sequentially one by one in resolved dependency or lexicographical order.
- **Graceful Pre-Stop Hook (`oops.stop.cmd`)**:
  - Before stopping or restarting any running container, inspect for the `oops.stop.cmd` label.
  - If present, execute the stop command inside the container with timeout `oops.stop.timeout` (default 30s).
  - If the hook times out or errors, log a warning and proceed with container termination.
- **Inter-Service Delay Gap (`--delay, -d <duration|int>`)**:
  - Optional flag with shorthand `-d` (e.g., `-d 5s`, `-d 5`, `--delay 5s`, or `--delay 5`) to introduce a pause between each consecutive service operation in a group.
  - Accepts standard duration strings (e.g. `5s`, `10s`, `1m`) or integer seconds (e.g. `5`), behaving intuitively like `sleep`.
  - Enables downstream consumers, load balancers, or connection pools to settle before the next service is transitioned.

### Sequential Rolling Update Algorithm (`oops update <targets...>`)
For each matched service in target order:
- **Step 1 - Pull**: Run `docker compose -f <group_compose> pull <service>`.
- **Step 2 - Stop Hook (if defined)**: If container has label `oops.stop.cmd`, execute the command inside the running container with timeout `oops.stop.timeout` (default `OOPS_STOP_TIMEOUT` or 30s).
- **Step 3 - Recreate**: Run `docker compose -f <group_compose> up -d --no-deps <service>`.
- **Step 4 - Health Polling**:
  - Poll Docker container inspection state `.State.Health.Status` every `OOPS_HEALTHCHECK_INTERVAL` (default 3s).
  - If `.State.Health.Status == "healthy"`: Service update succeeded. If `-d` / `--delay` is set, pause for the delay gap before advancing to next service.
  - If container has no native healthcheck but has `oops.health.url`: Send HTTP GET requests until 200 OK is received.
  - If status is `"unhealthy"` or polling exceeds `OOPS_HEALTHCHECK_TIMEOUT` (default 10m / 600s): Abort the update sequence immediately, print container logs, and exit with Code 1.

### Database Management (`oops db <mysql|postgres>[:<target>] <action>`)
- **Supported Engines**: `mysql`, `postgres`
- **Syntax**: `oops db mysql[:<target>] <create|passwd|list|drop> [args...]`
- **Target Resolution**:
  - `mysql` -> Default container `mysql`
  - `mysql:<target>` (e.g. `mysql:mysql-analytics`) -> Target container `mysql-analytics`
  - `postgres` -> Default container `postgres`
  - `postgres:<target>` (e.g. `postgres:pg-replica`) -> Target container `pg-replica`
- **Password Generation & Enforcement**: If password argument is omitted or empty (in `create` or `passwd`), generate a 20-character secure alphanumeric string (`[A-Za-z0-9]`). Never permit creating or updating users with blank passwords.
- **MySQL Queries**:
  - **Create**:
    ```sql
    CREATE DATABASE IF NOT EXISTS `<db>` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
    CREATE USER IF NOT EXISTS '<user>'@'%' IDENTIFIED BY '<pass>';
    GRANT ALL PRIVILEGES ON `<db>`.* TO '<user>'@'%';
    FLUSH PRIVILEGES;
    ```
  - **Passwd**:
    ```sql
    ALTER USER '<user>'@'%' IDENTIFIED BY '<new_pass>';
    FLUSH PRIVILEGES;
    ```
- **PostgreSQL Queries**:
  - **Create**:
    ```sql
    CREATE ROLE "<user>" WITH LOGIN PASSWORD '<pass>';
    CREATE DATABASE "<db>" OWNER "<user>";
    GRANT ALL PRIVILEGES ON DATABASE "<db>" TO "<user>";
    GRANT ALL ON SCHEMA public TO "<user>";
    ```
  - **Passwd**:
    ```sql
    ALTER ROLE "<user>" WITH PASSWORD '<new_pass>';
    ```

### Container Status Inspection (`oops status [targets...]`)
- **Container Discovery**: Connects to Docker Engine API (`/var/run/docker.sock`) to inspect active containers, health status, primary IP, and port bindings.
- **Port Formatting Rules**:
  - **Protocol Default**: TCP is default (e.g. `*:80`). Append `/udp` only for UDP ports (e.g. `*:53/udp`).
  - **Identical Port Simplification**: If host port equals container port (e.g. `80->80`), show `*:80` and omit `->80`. Use `->` only when ports differ (e.g. `*:[9001,9002]->9000`).
  - **Host IP Prefixes**: All-interface bindings (`0.0.0.0` / `::`) get `*:` prefix (`*:80`). Loopback bindings (`127.0.0.1` / `localhost`) get `#:` prefix (`#:8080`). Specific interface bindings get `<ip>:` prefix (`192.168.1.50:8080`).
  - **Deduplication**: Identical mappings across IPv4 and IPv6 are merged into a single entry (`*:80`).
  - **Multi-Port Grouping**: Multiple host ports mapped to the same container port are grouped into array notation (`*:[9001,9002,9003]->9000`).
  - **Internal Ports**: Unbound container ports are appended without parentheses (`*:80, *:443, 2019`).

### Automated Backup Suite (`oops backup`, `oops backup-db`, `oops backup-data`)
- **Unified Full Backup (`oops backup [-r <duration|int>]`)**:
  - Runs full system backup combining database dumps and filesystem volume archives.
  - Subcommand `oops backup prune` cleans all expired `.sql.gz` and `.tar.gz` files.
- **Database Backup (`oops backup-db [targets...] [-r <duration|int>]`)**:
  - Aliases: `oops db-backup`.
  - Targets: Dumps running database containers (`mysql`, `postgres`, `redis`).
  - Filename format: `backups/<engine>_backup_<TIMESTAMP>.sql.gz`.
  - Subcommand `oops backup-db prune` prunes expired database dump archives.
- **Filesystem & Volume Data Backup (`oops backup-data [names...] [-r <duration|int>]`)**:
  - Aliases: `oops data-backup`.
  - Targets: Reads `backups.data` targets from `oops.yml` (e.g. `uploads`, `certs`, `storage`).
  - Filename format: `backups/data_<name>_<TIMESTAMP>.tar.gz`.
  - Subcommand `oops backup-data prune` prunes expired data archive files.
- **Retention Pruning (`-r, --retention <duration|int>`)**:
  - Optional flag with shorthand `-r` (default `7d`, accepts `7d`, `14d`, `30`, `24h`).

### Working Directory Auto-Discovery & Path Parity
Oops commands can be invoked from any terminal directory. The working directory is determined using the strict hierarchy defined above:
- **Explicit Flag (`-C, --dir <path>`)**: Highest priority. Expands `~` to the user's home directory.
- **Current Directory (`.`)**: If `.` contains `stacks/` or compose files.
- **Active Box State (`~/.oops/active_box`)**: Automatically probes active workspace recorded on the workstation.
- **Environment Variable (`OOPSBOX_DIR`)**: If set in the shell (e.g. `export OOPSBOX_DIR=$HOME/oopsbox`).
- **Default Oopsbox Paths**: Auto-probes `$HOME/oopsbox` (Local Dev) and `/opt/oopsbox` (Production Server).

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops` | `[-C, --dir <path>]` | Displays help menu and available subcommands (default root execution) |
| `oops up` | `[targets...] [-d 0s]` | Starts stack or globbed services with optional inter-service delay (`-d`, `--delay`) |
| `oops stop` | `[targets...] [-x, --except, --exclude <tgt>] [-d 0s]` | Gracefully stops target services, or stops all other services except specified exclusion targets |
| `oops restart`| `[targets...] [-d 0s] [-i, --image <img/alias>]` | Restarts target services with stop hooks, delay gap, or image matching |
| `oops down` | `[-d 0s] [--wipe-all] [-y]` | Tears down all stacks with stop hooks and inter-service delay, or wipes all containers on Docker daemon (`--wipe-all`) |
| `oops switch` | `<target> [-d 0s]` | Switches active profile: starts target group/stack and stops all other running services |
| `oops status` | `[targets...]` | Formatted table of containers, health, and ports |
| `oops logs` | `[targets...] [--tail 50] [-f]` | Tail service logs across target services or stacks |
| `oops pull` | `[targets...] [--all]` | Pulls images for targets, default group, all stacks, or registry aliases |
| `oops update` | `[targets...] [--all] [-i, --image <img/alias>] [-d 0s]` | Executes sequential rolling update with health check and delay gap |
| `oops db` | `<mysql|postgres>[:<target>] <action>` | DB provisioning for `mysql` or `postgres`: `create`, `passwd`, `list`, `drop` |
| `oops backup` | `[-r 7d]` | Executes unified full backup (DB dumps + data volumes) and retention prune |
| `oops backup prune` | `[-r 7d]` | Prunes all expired backup archives (DB and data) |
| `oops backup-db` | `[targets...] [-r 7d]` | Executes DB dump and retention prune |
| `oops backup-db prune` | `[-r 7d]` | Prunes expired DB dump archives |
| `oops backup-data` | `[targets...] [-r 7d]` | Executes data volume tar archives and retention prune |
| `oops backup-data prune` | `[-r 7d]` | Prunes expired data volume archives |
| `oops storage check` | `` | Reports dead links and unmounted paths used by containers and backups; exits non-zero when found |
| `oops storage link` | `<target> [--name data] [--force]` | Links `<oopsbox>/data` to a persistent disk path after validation |
| `oops dns` | `[list]` | Inspects active DNS records, static mappings, and discovery routes |
| `oops dns add` | `<domain> <ip>` | Adds or updates static DNS record in `config/oops/dns` (supports `.wildcard`) |
| `oops dns del` | `<domain>` | Deletes static DNS record from `config/oops/dns` |
| `oops dns reload` | *(none)* | Validates `config/oops/dns` and triggers atomic reload across running daemon |
| `oops ip` | `[service]` | Displays the IP address of a target container or lists all IPs if omitted |
| `oops box create` | `<path>` | Creates new Oopsbox workspace from release blueprint, generates credentials, and sets active box |
| `oops box active` | `[path]` | Displays or sets the active Oopsbox workspace in `~/.oops/active_box` |
| `oops box list` | *(none)* | Lists known and active Oopsbox workspaces |
| `oops box start` | *(none)* | Boots VM engine (OrbStack/Colima/Docker), configures resolver, and starts default stack |
| `oops box stop` | *(none)* | Stops all running stacks via `oops down` on the active box |
| `oops box switch` | `<path>` | Gracefully tears down current box, updates active box, and starts target box |
| `oops box cert` | *(none)* | Installs local Caddy CA root certificate into host OS trust store / Keychain |
| `oops deploy` | `[-r <remote>] [ref]` | Deploys workspace updates to remote server over SSH |
| `oops remote add` | `<ssh-target> [-r <name>]` | Registers remote server via SSH (namespaced as `oops-<name>` in Git), sets up local Git remote |
| `oops remote list` | *(none)* | Lists registered remote servers (displays logical names without `oops-` prefix) |
| `oops remote remove` | `<name>` | Removes registered remote server configuration (`oops-<name>`) |
| `oops remote rename` | `<old-name> <new-name>` | Renames registered remote configuration from `oops-<old-name>` to `oops-<new-name>` |
| `oops rx` | `[-r <remote>] <cmd> [args...]` | Remote Execute: Forwards any `oops` command over SSH to remote server `~/oopsbox` using `-r <remote>` (default: `prod`) |
| `oops completion` | `[bash|zsh|fish|powershell]` | Generates shell auto-completion script for the specified shell |
| `oops version` | `[-v, --version]` | Displays the active Oops version, OS architecture, and build information |
| `oops selfupdate` | `[-c, --check] [-f, --force]` | Self-updates the oops binary to the latest release published on GitHub |
| `oops server` | `[-p, --port 8080] [-c, --config <path>]` | Starts webhook deployment and DNS discovery daemon |

---

## Binary Distribution & Multi-OS Releases

Upon tag releases (`v*.*.*`), standalone statically-linked executable binaries (`CGO_ENABLED=0`, `-ldflags="-s -w"`) and SHA-256 checksums (`checksums.txt`) are compiled and published directly to GitHub Release assets via GitHub Actions:
- `oops-linux-amd64`
- `oops-linux-arm64`
- `oops-darwin-amd64`
- `oops-darwin-arm64`

Release Mode is `host-release` (declared in `AGENTS.md`):
- **Release object**: Created by the release operator (`jarn-release`) from the matching `CHANGELOG.md` section, either before or after the tag push.
- **CI role**: The `Release Multi-OS Binaries` workflow builds assets, then polls (up to 10 minutes, every 10 seconds) until the release exists and uploads assets with `--clobber`. CI never creates the release, which prevents duplicates.
- **Timeout**: If the release is still missing, the job fails with an actionable error; create the release and re-run the failed job.
- **Version injection**: The tag `vX.Y.Z` is injected as `cmd.Version` via `-ldflags -X`.

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: CLI human operators, CI/CD runners (bash, scripts).
- **Downstream Dependencies**: Host Docker Engine API (`/var/run/docker.sock`), Docker Compose CLI, database engine binaries inside containers (`mysql`, `psql`, `redis-cli`).
- **Bounded Blast Radius**:
  - `cmd/`: CLI root and subcommands routing (`up`, `down`, `stop`, `restart`, `update`, `db`, `backup`, `status`, `logs`).
  - `internal/config/`: Configuration loader & environment variables.
  - `internal/docker/`: Docker SDK client, container inspector, compose resolver.
  - `internal/orchestrator/`: Rolling update engine, lifecycle coordinator, health watcher.
  - `internal/db/`: MySQL & Postgres provisioning helpers.
  - `internal/backup/`: Dump & retention manager.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./...`
- **Acceptance Scenarios**:
  - `oops up /<group>` correctly locates compose file in `edge/`, `db/`, `tool/`, `apps/`.
  - `oops up /db/mysql` starts only `mysql` service inside `db/compose.yml`.
  - `oops stop app..` executes `oops.stop.cmd` on all matching running containers sequentially with `--delay` (`-d`) gap.
  - `oops update app..` executes rolling update sequentially, waiting for health checks and observing delay gaps.
  - `oops db mysql create my_db my_user` generates 20-char password when omitted and creates isolated user.
  - `oops db postgres:pg-custom create my_db my_user my_pass` targets `pg-custom` container.
