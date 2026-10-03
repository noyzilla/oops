---
title: Unified DevOps Orchestration Engine
status: active
tags: [cli, webhook, docker, orchestration, rolling-update, database, backup]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "DESIGN.md"]
---

# Specification: Unified DevOps Orchestration Engine

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)

## Overview & Scope

Oops is a Unified DevOps Orchestration Binary & Container Platform (`ghcr.io/noyzilla/oops:latest`). It provides a single static binary running in two modes:
- **CLI Mode (`oops <command>`)**: Operator CLI to manage multi-layer Docker Compose stacks (`edge`, `db`, `apps`, `utils`), perform sequential rolling updates with health check polling, provision scoped database users, and automate database backups.
- **Server Mode (`oops server`)**: Background daemon receiving CI/CD webhooks with token authentication, graceful stop hooks, zero-downtime container recreation, and image pruning.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Stack**: The complete multi-layer setup in the active working directory.
- **Layer**: One of `edge`, `db`, `apps`, `utils`.
- **Service**: A named compose service in `docker-compose.yml`.
- **Target**: Glob pattern or exact name (`app*`, `db`, `mysql`).
- **Rolling Update**: Sequential pull -> recreate -> health poll workflow.

## Business Rules & Logic Invariants

### 1. Smart Layer Auto-Detection & Target Matching
- When `oops up`, `stop`, `restart`, `pull`, or `logs` is invoked with a target:
  - If target matches a layer name (`edge`, `db`, `apps`, `utils`), execute on that layer's `docker-compose.yml`.
  - If target is a glob pattern (`app*`, `*worker*`) or service name (`caddy`, `mysql`), scan all existing layer compose files, discover matching service names, and map them to their corresponding compose files.
  - If no target is specified, operate across all layers in topological order: `edge` -> `db` -> `apps` -> `utils` (for `up`/`pull`) and reverse for `down`.

### 2. Sequential Rolling Update Algorithm (`oops update <targets...>`)
For each matched service in target order:
- **Step 1 - Pull**: Run `docker compose -f <layer_compose> pull <service>`.
- **Step 2 - Stop Hook (if defined)**: If container has label `oops.stop.cmd`, execute the command inside the running container with timeout `oops.stop.timeout` (default 30s).
- **Step 3 - Recreate**: Run `docker compose -f <layer_compose> up -d --no-deps <service>`.
- **Step 4 - Health Polling**:
  - Poll Docker container inspection state `.State.Health.Status` every `HEALTHCHECK_INTERVAL_SECONDS` (default 3s).
  - If `.State.Health.Status == "healthy"`: Service update succeeded, advance to next service in sequence.
  - If container has no native healthcheck but has `oops.health.url`: Send HTTP GET requests until 200 OK is received.
  - If status is `"unhealthy"` or polling exceeds `HEALTHCHECK_TIMEOUT_SECONDS` (default 600s): Abort the update sequence immediately, print container logs, and exit with Code 1.

### 3. Database Management (`oops db <engine>[:<target>] <action>`)
- **Syntax**: `oops db mysql[:<target>] create <db> <user> [pass]` (also accepts `mysql/create` and `mysql-create`).
- **Target Resolution**:
  - `mysql` -> Default container `mysql`
  - `mysql:<target>` (e.g. `mysql:mysql-analytics`) -> Target container `mysql-analytics`
  - `pg` / `postgres` -> Default container `postgres`
  - `pg:<target>` (e.g. `pg:pg-replica`) -> Target container `pg-replica`
- **Password Enforcement**: If password argument is omitted or empty, generate a 20-character secure alphanumeric string (`[A-Za-z0-9]`). Never permit creating users with blank passwords.
- **MySQL Isolation**:
  ```sql
  CREATE DATABASE IF NOT EXISTS `<db>` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
  CREATE USER IF NOT EXISTS '<user>'@'%' IDENTIFIED BY '<pass>';
  GRANT ALL PRIVILEGES ON `<db>`.* TO '<user>'@'%';
  FLUSH PRIVILEGES;
  ```
- **PostgreSQL Isolation**:
  ```sql
  CREATE ROLE "<user>" WITH LOGIN PASSWORD '<pass>';
  CREATE DATABASE "<db>" OWNER "<user>";
  GRANT ALL PRIVILEGES ON DATABASE "<db>" TO "<user>";
  GRANT ALL ON SCHEMA public TO "<user>";
  ```

### 4. Database Backup & Retention (`oops backup [targets...]`)
- **Execution**:
  - `mysql`: `mysqldump --all-databases | gzip > backups/mysql_backup_<TIMESTAMP>.sql.gz`
  - `postgres`: `pg_dumpall | gzip > backups/postgres_backup_<TIMESTAMP>.sql.gz`
  - `redis`: Trigger `BGSAVE` and gzip snapshot `dump.rdb`
- **Retention**: Prune all backup files matching `<engine>_backup_*.sql.gz` older than `BACKUP_RETENTION_DAYS` (default 7 days).

### 5. Webhook Server Mode (`oops server`)
- **Daemon Invariants**:
  - Listens on `:8080` (or `$PORT`).
  - Auth header `X-Oops-Token: <SECRET>` matched against container's `oops.secret` label or global `OOPS_SECRET`.
  - Request body:
    ```json
    {
      "target": "app*",
      "mode": "image"
    }
    ```
  - Parses glob target and triggers rolling update engine on matching services.
  - Automatically runs dangling image cleanup (`docker image prune -f`) after successful deployment.

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops server` | `[--port 8080] [--config path]` | Starts webhook daemon |
| `oops up` | `[targets...]` | Starts stack, layer, or globbed services |
| `oops stop` | `<targets...>` | Gracefully stops target services |
| `oops restart`| `[targets...]` | Restarts target services |
| `oops down` | *(none)* | Tears down all layers |
| `oops status` | *(none)* | Formatted table of containers, health, and ports |
| `oops logs` | `<service> [--tail 100] [-f]` | Tail service logs |
| `oops pull` | `[targets...]` | Pulls images for targets |
| `oops update` | `<targets...>` | Executes sequential rolling update with health check |
| `oops db` | `<engine>[:<target>] <action>` | DB provisioning (`create`, `list`, `drop`) |
| `oops backup` | `[targets...] [--retention-days 7]` | Executes DB dump and retention prune |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: CLI human operators, CI/CD Webhook callers (GitHub Actions, GitLab CI).
- **Downstream Dependencies**: Host Docker Engine API (`/var/run/docker.sock`), Docker Compose CLI, database engine binaries inside containers (`mysql`, `psql`, `redis-cli`).
- **Bounded Blast Radius**:
  - `cmd/`: CLI root and subcommands routing.
  - `internal/config/`: Configuration loader & environment variables.
  - `internal/docker/`: Docker SDK client, container inspector, compose resolver.
  - `internal/orchestrator/`: Rolling update engine & health watcher.
  - `internal/db/`: MySQL & Postgres provisioning helpers.
  - `internal/backup/`: Dump & retention manager.
  - `internal/webhook/`: HTTP webhook listener & authentication.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./...`
- **Acceptance Scenarios**:
  - `oops up <layer>` correctly locates compose file in `edge/`, `db/`, `apps/`, `utils/`.
  - `oops update "app*"` executes rolling update sequentially, waiting for health checks.
  - `oops db mysql create my_db my_user` generates 20-char password when omitted and creates isolated user.
  - `oops db pg:pg-custom create my_db my_user my_pass` targets `pg-custom` container.
  - `POST /deploy` with valid token triggers deployment and prunes images.
