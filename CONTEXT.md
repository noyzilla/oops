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
- **Definition**: The complete host deployment environment encapsulating base compose manifests (`compose.yaml`), service templates (`services/`), configuration overlays (`config/`), live data storage (`data/`), backup dumps (`backups/`), and environment configuration (`.env`).
- **Standard Default Locations**: `~/oopsbox` (Local Dev) and `/opt/oopsbox` (Production Server).

### Entity: Group
- **Canonical Term**: `Group`
- **Definition**: A logical aggregation of service templates declared within `compose.yaml` under `x-oops-groups:` (e.g. `db: [mysql, redis]`).
- **CLI Notation**: `+<group>` (e.g. `+db`, `+monitoring`).
- **Avoid**: `bundle`, `cluster`, `environment`, `pod`

### Entity: Stack
- **Canonical Term**: `Stack`
- **Definition**: An application or stack composition defined in `compose.<stack>.yaml` at the project root, optionally paired with machine override `compose.<stack>.local.yaml`.
- **CLI Notation**: `/<stack>` (e.g. `/myapp`, `/crm`).
- **Avoid**: `project`, `project stack`, `bundle`, `app_instance`

### Entity: Capsule Blueprint
- **Canonical Term**: `Capsule Blueprint`
- **Definition**: Clean infrastructure and orchestrator templates (`compose.yaml.example`, `oops.yaml.example`, `.oopsallow.example`) used during `oops remote connect` to initialize a fresh remote capsule.

### Entity: Synchronization Whitelist
- **Canonical Term**: `Synchronization Whitelist` / `.oopsallow`
- **Definition**: A default-deny filter file residing in `/@<remote>/.oopsallow` that specifies the exact file patterns permitted for bidirectional synchronization (`oops remote push` and `oops remote pull`).

### Entity: Service Template
- **Canonical Term**: `Service Template`
- **Definition**: Pure, atomic service definitions residing in `services/<service>.yaml` (e.g. `services/mysql.yaml`, `services/redis.yaml`).
- **Permitted Synonyms**: `service_module`
- **Avoid**: `microservice`, `app_instance`

### Entity: Remote & Environment Overlay
- **Canonical Term**: `Remote` / `Environment Overlay`
- **Definition**: A self-contained target capsule residing at `/@<remote>/` (e.g. `/@prod/`, `/@staging/`) containing destination connection metadata (`remote.yaml`), untracked secrets (`.env`), and environment-specific compose/configuration overlays.
- **CLI Notation**: `@<remote>` (e.g. `@prod`, `@staging`).

### Entity: Config
- **Canonical Term**: `Config`
- **Definition**: Version-controlled service configurations residing modularly inside `config/<service>/` (e.g. `config/mysql/my.cnf`, `config/caddy/Caddyfile`). Tracked in Git and separate from raw binary storage (`data/`).

### Entity: Storage
- **Canonical Term**: `Storage`
- **Definition**: Physical host storage partitioned into two distinct persistence tiers:
  - `data/`: High-IOPS live realtime container data (`data/<service>/`).
  - `backups/`: Secondary backup storage (Cold storage / database dumps).

### Entity: Target
- **Canonical Term**: `Target`
- **Definition**: The user-supplied identifier passed to CLI commands, resolving to:
  - Exact service: `<service>` (e.g. `mysql`) -> auto-detects `services/<service>.yaml`.
  - Service group: `+<group>` (e.g. `+db`) -> resolves member templates from `x-oops-groups:`.
  - Application stack: `/<stack>` (e.g. `/myapp`) -> executes `compose.<stack>.yaml`.
  - Remote overlay: `@<remote>` (e.g. `@prod`) -> targets `/@<remote>/`.
- **Permitted Synonyms**: `target_pattern`, `target_selector`

### Concept: Rolling Update
- **Canonical Term**: `Rolling Update`
- **Definition**: Sequential zero-downtime deployment process where each matching service is pulled, recreated with `--no-deps`, and polled until its container health status is `healthy` before proceeding to the next service.

### Concept: Database Action
- **Canonical Term**: `Database Action`
- **Definition**: Operations managed by `oops db <engine>[:<target>] <action>` to create, list, drop, or change passwords for isolated databases and users with least-privilege credentials.
- **Permitted Canonical Values**:
  - `create`: Provision database and dedicated user with auto-generated 20-character secure password if omitted.
  - `passwd` (alias: `password`): Change or rotate password for an existing user (auto-generates 20-character password if omitted).
  - `list`: Inspect active databases and role grants.
  - `drop`: Safely remove database and associated user.

### Concept: Backup & Retention
- **Canonical Term**: `Backup`
- **Definition**: Operations managed by `oops db-backup [targets...] [-r <retention>]` to create compressed database dump archives (`.sql.gz` or `.rdb.gz`) saved under `BACKUP_DIR` with automated cleanup based on retention policy (`-r`, `--retention`).

---

## CLI Notation & Target Sigil Standards [มาตรฐานไวยากรณ์ Sigil และอาร์กิวเมนต์]

To ensure complete naming parity across CLI subcommands, developer documentation, and AI coding agents, all CLI argument placeholders must adhere to this unified standard:

### 1. Target Sigil Namespaces [ตารางกำหนด Sigil นำหน้า]

| Sigil | Domain Target | Meaning / Resolves To | Example Syntax |
| :--- | :--- | :--- | :--- |
| **`@`** | **Remote Host Capsule** | Target remote server capsule at `/@<remote>/` | `@prod`, `@staging`, `@192.168.1.50` |
| **`/`** | **Application Stack** | Dedicated application composition at `compose.<stack>.yaml` | `/myapp`, `/api`, `/worker` |
| **`+`** | **Service Group** | Aggregated service group declared in `compose.yaml` under `x-oops-groups:` | `+db`, `+monitoring` |gregated service group declared in `compose.yaml` under `x-groups:` | `+db`, `+monitoring` |
| **`.`** | **Internal Subsystem** | Internal system services (e.g. DNS daemon) | `.dns` |
| *(none)* | **Service Template** | Atomic reusable service definition at `services/<service>.yaml` | `mysql`, `redis`, `caddy` |

### 2. Argument Bracketing & Sigil-Outside Invariant [กฎการใช้วงเล็บและ Sigil อยู่นอกวงเล็บ]

- **Required Parameter Standard [พารามิเตอร์บังคับ]**:
  - Always written as `sigil<placeholder>` or `<placeholder>` (e.g. `@<remote>`, `/<stack>`, `<service>`).
  - Examples: `oops remote connect @<remote>[:<path>]`, `oops remote remove @<remote>`.
- **Optional Parameter Standard [พารามิเตอร์ทางเลือก]**:
  - Always wrapped in square brackets as `[sigil<placeholder>]` or `[placeholder]` (e.g. `[@<remote>]`, `[/<stack>]`, `[<service>]`).
  - Examples: `oops deploy [/<stack>] [@<remote>]`, `oops remote push [/<stack>] [@<remote>]`, `oops remote pull [/<stack>] [@<remote>]`, `oops rx [@<remote>] <cmd>`.
- **Sigil-Outside Invariant (Anti-HTML Collision Guard) [กฎ Sigil อยู่นอกวงเล็บแหลม]**:
  - Sigils (`@`, `/`, `+`) MUST strictly reside outside angle brackets `<...>`.
  - **Correct**: `[@<remote>]`, `[/<stack>]`, `[+<group>]`, `@<remote>`, `/<stack>`.
  - **Forbidden**: `[<@remote>]`, `[</stack>]`, `</stack>`. (Placing `/` inside `<...>` collides with HTML/XML closing tags like `</stack>` in Markdown renderers, syntax highlighters, and terminal parsers).

---

## Docker Labels Taxonomy

All Oops-managed container metadata is configured via Docker labels under the `oops.` namespace:

| Label Name | Type | Canonical Purpose |
| :--- | :--- | :--- |
| `oops.enable` | `bool` | Authorizes Oops to orchestrate, stop, and update this container (`"true"` / `"false"`). |
| `oops.secret` | `string` | Secret authentication token required for webhook deployments of this specific target. |
| `oops.health.url` | `string` | HTTP endpoint URL polled for 200 OK when native Docker healthcheck is absent. |

---

## Configuration & Environment Variables Parity

All environment variables follow the **[Topic] -> [Modifier] -> [Unit]** standard from `jarn-naming.md`.
Declarative defaults are loaded from `oops.yaml` (`x-oops:`), where tree paths map 1:1 to environment variables (`x-oops` -> `OOPS_`, tree separators -> `_`, UPPERCASE). Environment variables take precedence as overrides.

| Environment Variable | oops.yaml Tree Path | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `OOPSBOX_DIR` | (CLI Flag / Probe) | `string` | `""` | Target oopsbox directory (auto-probes `~/.oops/active_box`, `~/oopsbox`, `/opt/oopsbox`) |
| `OOPS_DOMAIN` | `services.oops.labels.caddy` | `string` | `""` | Optional ingress FQDN for webhook daemon |
| `OOPS_PORT` | (Internal Webhook) | `int / string` | `80` | Webhook HTTP daemon listening port |
| `OOPS_SECRET` | `x-oops.secret` | `string` | `""` | Global webhook secret token |
| `OOPS_DNS_TLD` | `x-oops.dns.tld` | `string` | `oops` | Local DNS top-level domain |
| `OOPS_DNS_UPSTREAMS` | `x-oops.dns.upstreams` | `string` | `1.1.1.1:53,8.8.8.8:53` | Upstream DNS relays (comma-separated or list) |
| `OOPS_HEALTHCHECK_TIMEOUT` | `x-oops.orchestrator.healthcheck.timeout` | `duration` | `10m` | Maximum wait time for container to become healthy |
| `OOPS_HEALTHCHECK_INTERVAL`| `x-oops.orchestrator.healthcheck.interval`| `duration` | `3s` | Polling interval between health checks |
| `OOPS_STOP_TIMEOUT` | `x-oops.orchestrator.stop_timeout` | `duration` | `30s` | Default timeout for graceful container stop |
| `OOPS_BACKUP_RETENTION` | `x-oops.backup.retention` | `duration` | `7d` | Retention window for database and data dump archives |
| `OOPS_BACKUP_DIR` | `x-oops.backup.dir` | `string` | `./backups` | Target directory for backup files |
| `OOPS_VOLUME_MOUNT_PREFIXES`| `x-oops.volume.mount_prefixes` | `string` | `/mnt,/media,/Volumes` | Permitted persistent storage mount prefixes |
| `OOPS_DB_MYSQL_PASSWORD` | `x-oops.db.mysql_password` | `string` | `""` | Superuser password for MySQL |
| `OOPS_DB_POSTGRES_PASSWORD` | `x-oops.db.postgres_password` | `string` | `""` | Superuser password for Postgres |
| `OOPS_DB_REDIS_PASSWORD` | `x-oops.db.redis_password` | `string` | `""` | Superuser / requirepass password for Redis |

---

## Maintenance Guidelines

- **Consult Before Naming**: When introducing new CLI commands, flags, or configuration keys, ensure alignment with terms defined in this glossary.
- **Enforce Code Reviews**: Reject pull requests introducing deprecated synonyms, camelCase CLI flags, or non-standard environment variable prefixes.
