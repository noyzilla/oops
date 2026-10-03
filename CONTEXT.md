# Domain Context & Ubiquitous Language

This document establishes the official domain glossary and ubiquitous language for the **oops** project. Both human engineers and AI coding assistants must adhere to these definitions across source code, CLI subcommands, database helpers, webhook APIs, tests, and documentation.

## Purpose & Authority

- **Eliminate Naming Drift**: Prevent AI agents and contributors from introducing conflicting terms or synonyms for the same domain entity.
- **Consistent Code Identifiers**: Variable names, function identifiers, CLI flags, configuration fields, and API parameters must strictly mirror the terms defined here.
- **Single Source of Truth**: When domain requirements evolve, update this document before refactoring codebase identifiers.

---

## Core Domain Entities & Concepts

### Entity: Oopsbox
- **Canonical Term**: `Oopsbox`
- **Definition**: The complete host deployment environment encapsulating all stacks (`stacks/`), live data storage (`data/`), backup dumps (`backups/`), and environment configuration (`.env`).
- **Standard Default Locations**: `~/oopsbox` (Local Dev) and `/opt/oopsbox` (Production Server).

### Entity: Stack
- **Canonical Term**: `Stack`
- **Definition**: A distinct functional tier and docker compose setup residing in `stacks/<stack>/` (or top-level compose) containing a dedicated `docker-compose.yml` file.
- **Permitted Canonical Values**:
  - `edge`: Ingress reverse proxy and SSL termination (e.g. Caddy, Traefik).
  - `db`: Persistence and cache databases (e.g. MySQL, PostgreSQL, Redis).
  - `utils`: Auxiliary daemons, sidecars, proxies, queues, and oops webhook engine.
  - `apps`: Core application services and web backends.
  - `[custom]`: Dynamic custom stacks (e.g. `monitoring`, `analytics`, `ai`).
- **Dependency Startup Order**: `edge` -> `db` -> `utils` -> `apps` -> `[custom...]`
- **Avoid**: `bundle`, `cluster`, `environment`, `layer` (legacy term), `pod`

### Entity: Config
- **Canonical Term**: `Config`
- **Definition**: Version-controlled system and service configurations residing in `config/` (e.g. `config/oops/hosts`, `config/caddy/Caddyfile`, `config/mysql/my.cnf`). Tracked in Git and separate from raw binary database storage (`data/`).

### Entity: Storage
- **Canonical Term**: `Storage`
- **Definition**: Physical host storage partitioned into two distinct persistence tiers:
  - `data/`: High-IOPS live realtime container data (NVMe / SSD persistent volumes).
  - `backups/`: Secondary backup storage (Cold storage / database dumps).

### Entity: Service
- **Canonical Term**: `Service`
- **Definition**: A named workload definition inside a `docker-compose.yml` file (e.g. `caddy`, `mysql`, `app1`).
- **Permitted Synonyms**: `compose_service`
- **Avoid**: `module`, `microservice`, `app_instance`

### Entity: Target
- **Canonical Term**: `Target`
- **Definition**: The user-supplied identifier passed to CLI commands or Webhook payloads, resolving to stacks via slash prefix (`/apps`, `/db`), scoped services via path (`/db/mysql`), exact service names (`mysql`), or Double Dot wildcards (e.g. `app..`, `..worker`, `..api..`).
- **Permitted Synonyms**: `target_pattern`, `target_selector`
- **Avoid**: `regex_pattern`, `filter_query`

### Concept: Rolling Update
- **Canonical Term**: `Rolling Update`
- **Definition**: Sequential zero-downtime deployment process where each matching service is pulled, recreated with `--no-deps`, and polled until its container health status is `healthy` before proceeding to the next service.
- **Permitted Synonyms**: `sequential_update`, `rolling_upgrade`
- **Avoid**: `blue_green`, `hot_swap`, `batch_restart`

### Concept: Database Action
- **Canonical Term**: `Database Action`
- **Definition**: Operations managed by `oops db <engine>[:<target>] <action>` to create, list, drop, or change passwords for isolated databases and users with least-privilege credentials.
- **Permitted Canonical Values**:
  - `create`: Provision database and dedicated user with auto-generated 20-character secure password if omitted.
  - `passwd` (alias: `password`): Change or rotate password for an existing user (auto-generates 20-character password if omitted).
  - `list`: Inspect active databases and role grants.
  - `drop`: Safely remove database and associated user.
- **Avoid**: `add_db`, `remove_db`, `grant_user`, `change_pass`

### Concept: Backup & Retention
- **Canonical Term**: `Backup`
- **Definition**: Operations managed by `oops db-backup [targets...] [-r <retention>]` to create compressed database dump archives (`.sql.gz` or `.rdb.gz`) saved under `BACKUP_DIR` with automated cleanup based on retention policy (`-r`, `--retention`).
- **Permitted Synonyms**: `snapshot`, `dump`
- **Avoid**: `export`, `replica`

---

## Docker Labels Taxonomy

All Oops-managed container metadata is configured via Docker labels under the `oops.` namespace:

| Label Name | Type | Canonical Purpose |
| :--- | :---: | :--- |
| `oops.enable` | `bool` | Authorizes Oops to orchestrate, stop, and update this container (`"true"` / `"false"`). |
| `oops.secret` | `string` | Secret authentication token required for webhook deployments of this specific target. |
| `oops.stop.cmd` | `string` | Custom command executed inside the container prior to stopping. |
| `oops.stop.timeout` | `int` | Grace period in seconds to wait for `oops.stop.cmd` before sending `SIGTERM`. |
| `oops.health.url` | `string` | HTTP endpoint URL polled for 200 OK when native Docker healthcheck is absent. |

---

## Configuration & Environment Variables Parity

All environment variables follow the **[Topic] -> [Modifier] -> [Unit]** standard from `jarn-naming.md`:

| Environment Variable | Internal Config Path | Type | Default | Description |
| :--- | :--- | :---: | :---: | :--- |
| `OOPS_DIR` | `CONFIG.CLI.WORK_DIR` | `string` | `""` | Target oopsbox directory (auto-probes `~/oopsbox`, `/opt/oopsbox`) |
| `OOPS_DOMAIN` | `CONFIG.SERVER.DOMAIN` | `string` | `""` | Optional ingress FQDN for webhook daemon |
| `OOPS_PORT` | `CONFIG.SERVER.PORT` | `int / string` | `80` | Webhook HTTP daemon listening port (fallback: `PORT`) |
| `OOPS_SECRET` | `CONFIG.WEBHOOK.SECRET` | `string` | `""` | Global fallback webhook secret token |
| `OOPS_DNS_UPSTREAM` | `CONFIG.DNS.UPSTREAM` | `string` | `1.1.1.1:53,8.8.8.8:53` | Upstream DNS relays for non-local domain queries |
| `OOPS_HEALTHCHECK_TIMEOUT` | `CONFIG.HEALTHCHECK.TIMEOUT` | `duration` | `10m` | Maximum wait time for container to become healthy (fallback: `HEALTHCHECK_TIMEOUT_SECONDS`) |
| `OOPS_HEALTHCHECK_INTERVAL`| `CONFIG.HEALTHCHECK.INTERVAL`| `duration` | `3s` | Polling interval between health checks (fallback: `HEALTHCHECK_INTERVAL_SECONDS`) |
| `OOPS_STOP_TIMEOUT` | `CONFIG.CONTAINER.STOP_TIMEOUT`| `duration` | `30s` | Default timeout for graceful container stop (fallback: `STOP_TIMEOUT_SECONDS`) |
| `OOPS_BACKUP_RETENTION` | `CONFIG.BACKUP.RETENTION` | `duration` | `7d` | Retention window for database dump archives (fallback: `BACKUP_RETENTION_DAYS`) |
| `OOPS_BACKUP_DIR` | `CONFIG.BACKUP.DIR` | `string` | `./backups` | Target directory for backup files (fallback: `BACKUP_DIR`) |
| `MYSQL_ROOT_PASSWORD` | `CONFIG.MYSQL.ROOT_PASSWORD` | `string` | `""` | Root password for MySQL container exec |
| `POSTGRES_PASSWORD` | `CONFIG.POSTGRES.PASSWORD` | `string` | `""` | Admin password for Postgres exec |

---

## Maintenance Guidelines

- **Consult Before Naming**: When introducing new CLI commands, flags, or configuration keys, ensure alignment with terms defined in this glossary.
- **Enforce Code Reviews**: Reject pull requests introducing deprecated synonyms, camelCase CLI flags, or non-standard environment variable prefixes.
