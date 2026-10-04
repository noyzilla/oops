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

Oops provides an operator CLI binary (`oops <command>`) designed to manage multi-stack Docker Compose environments (`stacks/edge`, `stacks/db`, `stacks/apps`, `stacks/utils`), execute sequential rolling updates with health check polling, enforce graceful lifecycle shutdown hooks, provision least-privilege database credentials, and manage automated database backups.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Stack**: A cohesive group of services defined in a `compose.yml` (or compose file) under `stacks/<stack>/` (e.g. `/edge`, `/db`, `/apps`, `/utils`, or custom sub-directories).
- **Service**: A named compose service in `compose.yml`.
- **Target**: Explicit stack (`/db`, `/apps`), scoped service (`/db/mysql`, `/apps/api`), exact service name (`caddy`, `mysql`), or Double Dot wildcard (`app..`, `..worker`, `..api..`).
- **Rolling Update**: Sequential pull -> stop hook -> recreate -> health poll workflow.

## Business Rules & Logic Invariants

### Smart Target Resolution & Double Dot (`..`) Wildcard Protocol
Target strings are resolved using an explicit, shell-safe notation that eliminates quoting overhead on physical and virtual keyboards:
- **Group Stacks (`@<group>`)**:
  - Any target starting with `@` (e.g., `@core`, `@pg`, `@minimal`, `@all`) resolves against `stacks/oops.yml` (or `oops.yml`). It expands recursively into the constituent stacks and services defined in the named profile.
- **Registry Aliases & Image Matching (`<alias>/<image>:<tag>` or `<image>:<tag>`)**:
  - Any target matching a configured registry alias (e.g. `gar/web-app:v1.0` -> `asia-southeast1-docker.pkg.dev/.../web-app:v1.0`), `img:<image>`, or exact image name resolves to all services across all stacks using that container image.
- **Stack Target (`/<stack>`)**:
  - Any target starting with a leading slash `/` without further subpaths (e.g., `/edge`, `/db`, `/apps`, `/utils`) targets the entire stack and executes on that stack's `compose.yml` (located under `stacks/<stack>/`).
- **Scoped Service Target (`/<stack>/<service>`)**:
  - Path notation (e.g., `/db/mysql`, `/apps/web`) targets only the specified service strictly inside the designated stack.
- **Exact Service Name (Bare string without `..`)**:
  - A bare string without `..` (e.g., `mysql`, `caddy`) strictly matches only an exact service name. If no exact match exists, an error is returned.
- **Double Dot (`..`) Wildcard Matcher**:
  - **Prefix Match (`<prefix>..`)**: e.g., `app..` matches all services starting with `app` (e.g. `app-web`, `app-worker`).
  - **Suffix Match (`..<suffix>`)**: e.g., `..worker` matches all services ending with `worker` (e.g. `mail-worker`, `job-worker`).
  - **Contains Match (`..<keyword>..`)**: e.g., `..api..` matches any service name containing `api`.
- **Default Group & Global Stack (Omitted Target)**:
  - If no target is specified, the CLI checks for `OOPS_DEFAULT_GROUP` (from shell environment or `.env` in the working directory). If set (e.g. `OOPS_DEFAULT_GROUP=core`), it defaults to starting the specified `@group`.
  - If `OOPS_DEFAULT_GROUP` is not set, operations execute across all stacks in deterministic dependency order:
    - **Startup (`oops up`)**: `/edge` -> `/db` -> `/utils` -> `/apps` -> `[custom stacks...]`
    - **Teardown (`oops down`)**: `[custom stacks...]` -> `/apps` -> `/utils` -> `/db` -> `/edge`

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
- **Step 1 - Pull**: Run `docker compose -f <layer_compose> pull <service>`.
- **Step 2 - Stop Hook (if defined)**: If container has label `oops.stop.cmd`, execute the command inside the running container with timeout `oops.stop.timeout` (default `OOPS_STOP_TIMEOUT` or 30s).
- **Step 3 - Recreate**: Run `docker compose -f <layer_compose> up -d --no-deps <service>`.
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

### Database Backup & Retention (`oops db-backup [targets...]`)
- **Syntax**: `oops db-backup [targets...] [-r <duration|int>]` and `oops db-backup prune [-r <duration|int>]`.
- **Target Resolution**:
  - If targets omitted: Dumps all running database containers discovered in the `db` layer (`mysql`, `postgres`, `redis`).
  - If target specified (e.g. `mysql`, `pg`, `pg:replica`): Dumps only the matching database container.
- **Execution**:
  - `mysql`: `mysqldump --all-databases | gzip > backups/mysql_backup_<TIMESTAMP>.sql.gz`
  - `postgres`: `pg_dumpall | gzip > backups/postgres_backup_<TIMESTAMP>.sql.gz`
  - `redis`: Trigger `BGSAVE` and gzip snapshot `dump.rdb`
- **Retention Pruning (`-r, --retention <duration|int>`)**:
  - Optional flag with shorthand `-r` (default `7d`, accepts `7d`, `30d`, or integer days `7`).
  - Prunes all backup files matching `<engine>_backup_*.sql.gz` older than the retention threshold.

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
| `oops stop` | `<targets...> [-d 0s]` | Gracefully stops target services with stop hooks and inter-service delay (`-d`, `--delay`) |
| `oops restart`| `[targets...] [-d 0s] [-i, --image <img/alias>]` | Restarts target services with stop hooks, delay gap, or image matching |
| `oops down` | `[-d 0s]` | Tears down all stacks with stop hooks and inter-service delay (`-d`, `--delay`) |
| `oops status` | `[targets...]` | Formatted table of containers, health, and ports |
| `oops logs` | `[targets...] [--tail 100] [-f]` | Tail service logs across target services or stacks |
| `oops pull` | `[targets...] [--all]` | Pulls images for targets, default group, all stacks, or registry aliases |
| `oops update` | `[targets...] [--all] [-i, --image <img/alias>] [-d 0s]` | Executes sequential rolling update with health check and delay gap |
| `oops db` | `<engine>[:<target>] <action>` | DB provisioning (`create`, `passwd`, `list`, `drop`) |
| `oops db-backup`| `[targets...] [-r 7d]` | Executes DB dump and retention prune (`-r`, `--retention`) |
| `oops db-backup prune`| `[-r 7d]` | Prunes expired backup archives without triggering a new dump |
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
  - `oops up /<layer>` correctly locates compose file in `edge/`, `db/`, `apps/`, `utils/`.
  - `oops up /db/mysql` starts only `mysql` service inside `db/docker-compose.yml`.
  - `oops stop app..` executes `oops.stop.cmd` on all matching running containers sequentially with `--delay` (`-d`) gap.
  - `oops update app..` executes rolling update sequentially, waiting for health checks and observing delay gaps.
  - `oops db mysql create my_db my_user` generates 20-char password when omitted and creates isolated user.
  - `oops db pg:pg-custom create my_db my_user my_pass` targets `pg-custom` container.
