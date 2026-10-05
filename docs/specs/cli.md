---
title: CLI Orchestration & Stack Management
status: active
tags: [cli, docker, compose, rolling-update, lifecycle-hooks, database, backup]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "DESIGN.md", "docs/specs/webhook.md"]
---

# Specification: CLI Orchestration & Stack Management

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Spec**: [docs/specs/webhook.md](webhook.md)

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
| **3** | **Environment Variables (`OOPSBOX_DIR` / `OOPS_DIR`)** | Outside an Oopsbox directory (`hasComposeContent(".") == false`) | **Global Command Execution**: Allows executing `oops` commands from arbitrary directories (e.g. `~`, `~/projects/my-app`, `/tmp`) while targeting a specific registered Oopsbox hub. |
| **4** | **Standard Workstation Default (`$HOME/oopsbox`)** | `hasComposeContent("$HOME/oopsbox") == true` | Standard turnkey installation path on macOS and developer workstations. |
| **5** | **Standard Server Default (`/opt/oopsbox`)** | `hasComposeContent("/opt/oopsbox") == true` | Standard turnkey production deployment path on Linux cloud servers. |
| **6** | **Fallback (`.`)** | None of the above matched | Returns `.` where subsequent validation (`DiscoverStacks`) returns a clear error if not an Oopsbox directory. |

#### Two-Stage Workspace Validation Invariants
1. **Pre-Flight Inspection (`hasComposeContent`)**: Validates that candidate directories are accessible and contain either a `stacks/` subdirectory or compose manifests (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`).
2. **Execution Guard (`DiscoverStacks`)**: When executing lifecycle commands (`up`, `stop`, `restart`, `update`, `db`, `backup`), validates that at least one valid compose stack is discovered. If zero stacks exist, execution halts immediately with:
   `Error: no compose stacks or services found in workspace "<workDir>"`.

#### Docker Wrapper Same-Host-Path Binding Invariant
When running the Oops CLI via Docker wrapper container (`ghcr.io/noyzilla/oops:latest`), the host directory MUST be bound to the **identical absolute path** inside the container (`-v "${OOPSBOX_DIR}":"${OOPSBOX_DIR}" -w "${OOPSBOX_DIR}"`):
- **Why `/workspace` binding is forbidden**: Docker Compose passes working directory labels (`com.docker.compose.project.working_dir`) and relative volume bind mounts (`./caddy/Caddyfile`) to the host Docker daemon. If bound to `/workspace`, the host daemon attempts to locate `/workspace` on the host, causing path mismatches and volume failures. Same-host-path binding ensures 100% path parity with direct host execution.

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

### Database Management (`oops db <engine>[:<target>] <action>`)
- **Syntax**: `oops db mysql[:<target>] <create|passwd|list|drop> [args...]` (also accepts shorthand `mysql/create`, `mysql-create`, `mysql passwd`, `mysql password`).
- **Target Resolution**:
  - `mysql` -> Default container `mysql`
  - `mysql:<target>` (e.g. `mysql:mysql-analytics`) -> Target container `mysql-analytics`
  - `pg` / `postgres` -> Default container `postgres`
  - `pg:<target>` (e.g. `pg:pg-replica`) -> Target container `pg-replica`
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
Oops commands can be invoked from any terminal directory. The working directory is determined using a 4-tier fallback:
- **Explicit Flag (`-C, --dir <path>`)**: Highest priority. Expands `~` to the user's home directory.
- **Environment Variable (`OOPS_DIR`)**: If set in the shell (e.g. `export OOPS_DIR=$HOME/oopsbox`).
- **Current Directory (`.`)**: If `.` contains `stacks/` or compose files.
- **Default Oopsbox Paths**: Auto-probes `$HOME/oopsbox` (Local Dev) and `/opt/oopsbox` (Production Server).

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops` | `[-C, --dir <path>]` | Global persistent flag to target a specific oopsbox directory |
| `oops up` | `[targets...] [-d 0s]` | Starts stack or globbed services with optional inter-service delay (`-d`, `--delay`) |
| `oops stop` | `[targets...] [-x, --except, --exclude <tgt>] [-d 0s]` | Gracefully stops target services, or stops all other services except specified exclusion targets |
| `oops restart`| `[targets...] [-d 0s] [-i, --image <img/alias>]` | Restarts target services with stop hooks, delay gap, or image matching |
| `oops down` | `[-d 0s]` | Tears down all stacks with stop hooks and inter-service delay (`-d`, `--delay`) |
| `oops switch` | `<target> [-d 0s]` | Switches active profile: starts target group/stack and stops all other running services |
| `oops status` | `[targets...]` | Formatted table of containers, health, and ports |
| `oops logs` | `[targets...] [--tail 100] [-f]` | Tail service logs across target services or stacks |
| `oops pull` | `[targets...] [--all]` | Pulls images for targets, default group, all stacks, or registry aliases |
| `oops update` | `[targets...] [--all] [-i, --image <img/alias>] [-d 0s]` | Executes sequential rolling update with health check and delay gap |
| `oops db` | `<engine>[:<target>] <action>` | DB provisioning (`create`, `passwd`, `list`, `drop`) |
| `oops backup` | `[-r 7d]` | Executes unified full backup (DB dumps + data volumes) and retention prune |
| `oops backup prune` | `[-r 7d]` | Prunes all expired backup archives (DB and data) |
| `oops backup-db` | `[targets...] [-r 7d]` | Executes DB dump and retention prune (alias: `db-backup`) |
| `oops backup-db prune` | `[-r 7d]` | Prunes expired DB dump archives |
| `oops backup-data` | `[targets...] [-r 7d]` | Executes data volume tar archives and retention prune (alias: `data-backup`) |
| `oops backup-data prune` | `[-r 7d]` | Prunes expired data volume archives |
| `oops dns` | `[list]` | Inspects active DNS records, static mappings, and discovery routes |
| `oops dns add` | `<domain> <ip>` | Adds or updates static DNS record in `config/oops/dns` (supports `.wildcard`) |
| `oops dns del` | `<domain>` | Deletes static DNS record from `config/oops/dns` |
| `oops dns reload` | *(none)* | Validates `config/oops/dns` and triggers atomic reload across running daemon |
| `oops server` | `[-p, --port 80] [-c, --config <path>]` | Starts webhook deployment and DNS discovery daemon (aliases: `webhook`, `daemon`) |

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
  - `oops db pg:pg-custom create my_db my_user my_pass` targets `pg-custom` container.
