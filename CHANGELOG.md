# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project adheres to Semantic Versioning.

## [Unreleased]

## [v1.24.0] - 2026-10-25

### Added
- **macOS DNS Resolver Repair (`oops dns repair`, alias `fix`)**: Added command to inspect all active workspace and remote resolvers in `/etc/resolver/` and synchronize their `nameserver` IPs to the active `edge/oops` container IP, with automatic macOS DNS cache flushing (`dscacheutil -flushcache`) and `--dry-run` preview support (`cmd/dns.go`, `internal/box/engine.go`, `docs/specs/cli.md`, `docs/specs/dns.md`).
- **Doctor Stale Resolver Nameserver IP Audit**: Enhanced `oops doctor` and `oops doctor dns` to audit the `nameserver` IP inside `/etc/resolver/*` files against the active `edge/oops` container IP, alerting on outdated/stale resolver configurations and directing users to `oops dns repair` (`internal/doctor/inspector.go`).

## [v1.23.0] - 2026-10-25

### Changed
- **Stack Path Notation in `oops ps`**: Standardized the `STACK` column to use path notation: `/` for root compose manifests (`compose.yaml`), `/<stack>` for sub-stacks (`/db`, `/edge`, etc.), `/tunnel` for tunnel containers (with remote name in `SERVICE`), and `%` for external containers. Added ordered table sorting (`/` first, named stacks alphabetically, `%` external last) and fixed root compose string-trimming bug (`cmd/ps.go`, `docs/specs/cli.md`).
- **Default Redis 8 Blueprint (`redis:8-alpine`)**: Updated the default Redis stack image and restore fallback from `redis:7-alpine` to `redis:8-alpine`, providing native RedisJSON, RediSearch capabilities, and rolling security updates (`oopsbox/compose.db.yaml`, `internal/backup/restore.go`).

## [v1.22.0] - 2026-10-25

### Added
- **Database Credential Repair Action (`oops db <engine> repair [user]`)**: Added `repair` action to inspect `config/env/` credential files and synchronize running database container users and passwords with `.env` files across MySQL, PostgreSQL, and Redis (`cmd/db.go`, `internal/db/`, `docs/specs/cli.md`).
- **Duplicate Database & User Creation Protection**: Added pre-creation existence checks across MySQL, PostgreSQL, and Redis in `oops db create` and `readonly`, failing fast if the database or user already exists and providing explicit guidance to use `oops db <engine> passwd <user>` (`cmd/db.go`, `internal/db/`, `docs/specs/cli.md`).
- **Redis ACL Client Ready Check Support (`+info`)**: Added `+info` permission to Redis ACL generation (`BuildRedisCreateACL` and `BuildRedisReadonlyACL`), enabling standard client library connection handshakes (such as ioredis `enableReadyCheck`) without opening server-blocking commands like `KEYS` (`internal/db/redis.go`, `docs/specs/cli.md`).

### Fixed
- **Redis Stale Password Purge on `passwd` (`resetpass`)**: Fixed `BuildRedisPasswdACL` to include `resetpass` before setting a new password, purging legacy multi-password accumulation and guaranteeing `+info` permission on rotated Redis users (`internal/db/redis.go`, `docs/specs/cli.md`).
- **Redis Prefix Terminology in CLI (`oops db redis`)**: Replaced `Database:` label and `<db_name>` parameter terminology with `Prefix:` and `<prefix>` across `create`, `readonly`, and `drop` subcommands, reflecting Redis key-pattern ACLs and lifting the 2-level underscore restriction for Redis namespaces (`cmd/db.go`, `docs/specs/cli.md`).

## [v1.21.0] - 2026-10-25

### Changed
- **Pure Engine-Scoped Database Environment Templates**: Replaced generic `DB_*` environment templates with pure, collision-free engine-scoped variables (`MYSQL_*`, `POSTGRES_*`, `REDIS_*`). Redis `.env` files now cleanly provide `REDIS_PREFIX` and `REDIS_DB=0` without conflicting `DB_NAME` or `DB_PASSWORD` variables (`cmd/db.go`, `docs/specs/cli.md`).

## [v1.20.0] - 2026-10-25

### Added
- **Structured Database CLI Help (`oops db`)**: Separated supported engines (`mysql`, `postgres`, `redis`) and provisioning actions (`create`, `readonly`, `passwd`, `list`, `drop`) into distinct formatted sections with practical usage examples (`cmd/db.go`).

## [v1.19.0] - 2026-10-25

### Added
- **Compact Trusted Core Subnet (`10.240.0.0/24`) & IPAM Range**: Adopted compact `/24` subnet with deterministic dynamic IP range (`ip_range: 10.240.0.128/25`), reserving `10.240.0.2` – `10.240.0.127` for static core services (`oops`, `caddy-proxy`) without risk of dynamic IP allocation collision.
- **Multi-Host Subnet Sizing Policy in `oops.yaml`**: Standardized default `x-oops.network.subnet: 10.240.0.0/24` with comprehensive inline documentation covering single-network host defaults, multi-remote mesh routing without route collision (`@server1: 10.240.0.0/24`, `@server2: 10.240.1.0/24`), and multi-network expansion (`/16`).
- **Oops Daemon IP Precedence**: Placed `oops` service definition ahead of `caddy-proxy` in `compose.edge.yaml` with fixed IP `10.240.0.2` and updated doctor compose to recognize `.2` reservation for `oops`.

### Changed
- **Clean Compose Architecture**: Removed redundant service-level `networks: - default` and root-level `networks:` blocks from stack manifests (`compose.edge.yaml`, `compose.db.yaml`, `compose.tools.yaml`, `compose.storage.yaml`), relying on Docker Compose's automatic `default` network inheritance.
- **Storage Stack Unification**: Replaced separate `oops_storage` network with the trusted core `default` (`oops_default`) network for `seaweedfs`.

## [v1.18.0] - 2026-10-24

### Added
- **Mesh VPN L3 Subnet & IPAM Schema**: Introduced deterministic `network.subnet` in `remote.yaml` (default `10.240.0.0/16`) and root `compose.yaml` `x-oops.network.subnet`, with IP containment validation (`ValidateNetworkSubnet`, `IsSubnetContained`) and deterministic IP reservations (`.1` Gateway, `.2` DNS) (`internal/docker/config.go`, `internal/remote/blueprint.go`, `docs/specs/remote.md`).
- **Single-TLD Remote Identity & Automated Compose Service Registration**: Enforced `dns.tld == <remote_name>` on every capsule, automatically registering `com.docker.compose.service` as `<service>.<tld>` alongside `<service>.oops`, paired TLD routing, and Zero Public Leakage Guard with instant NXDOMAIN fail-fast (`internal/dns/resolver.go`, `internal/dns/watcher.go`, `internal/dns/server.go`, `docs/specs/dns.md`).
- **macOS Resolver Lifecycle & Pruning (`oops dns prune`)**: One-time sudo configuration for `/etc/resolver/<remote>` during `oops remote connect`, automated removal/renaming during remote lifecycle, and `oops dns prune` (alias `fix`) to purge stale resolver files (`internal/box/engine.go`, `cmd/remote.go`, `cmd/dns.go`, `docs/specs/dns.md`).
- **All-in-One Health Diagnostic Suite (`oops doctor`)**: Implemented `oops doctor`, `oops doctor compose [@<remote> | --all]`, and `oops doctor dns` to audit Compose subnet containment, cross-remote subnet collisions, static IP reservations, and orphaned resolvers (`internal/doctor/inspector.go`, `cmd/doctor.go`, `docs/specs/cli.md`).
- **Dual-Mode Tunnel Runner (`port` vs `mesh`)**: Added support for `tunnel.mode: mesh | port` in `remote.yaml` and CLI flags `--mesh` / `--port`, establishing dynamic SOCKS5 routing over SSH for transparent L3 container communication without local port clashes (`internal/tunnel/config.go`, `internal/tunnel/container.go`, `internal/tunnel/manager.go`, `cmd/tunnel.go`, `docs/specs/tunnel.md`).

## [v1.17.0] - 2026-10-24

### Added
- **Capsule Standard Directory Hierarchy (`config`, `data`, `backups`)**: Initialized standard Oopsbox directories (`config/`, `data/`, `backups/`) in remote capsules (`/@<remote>/`) to fully support `oops backup` operations and operational directory parity (`internal/remote/blueprint.go`, `internal/remote/config_test.go`, `docs/specs/remote.md`).
- **Commented Tunnel Template in `remote.yaml`**: Automatically generates commented `tunnel:` configuration blueprint with service discovery examples in `remote.yaml` on new remote connections (`internal/remote/config.go`, `internal/remote/config_test.go`, `docs/specs/remote.md`).

### Fixed
- **Safe Empty `x-oops-stack` Blueprint Default**: Commented out `- /edge` and `- /db` by default in `compose.yaml.example` and `defaultComposeExample`, preventing missing stack errors when initializing fresh capsules or workspaces (`oopsbox/compose.yaml.example`, `internal/remote/blueprint.go`).

## [v1.16.0] - 2026-10-24

### Changed
- **Multiline Tunnel Endpoints Display with Scoped Service Targets**: Replaced comma-separated single-line endpoints in `oops tunnel list` with multiline endpoint rows. Dynamically resolves service targets against local workspace stacks, displaying `endpoint (/<stack>/<service>)` (e.g., `mysql.b9tpe.tunnel.oops:3306 (/db/mysql)`) and dropping the redundant `FORWARDED` column (`cmd/tunnel.go`, `cmd/tunnel_test.go`, `docs/specs/tunnel.md`).

## [v1.15.0] - 2026-10-24

### Added
- **Smart SSH Config & External Key Mount Inspector (`ResolveSSHReferencedDirectories`)**: Dynamically resolves OpenSSH `IdentityFile`, `CertificateFile`, and `Include` targets via `ssh -G <host>` and parses `~/.ssh/config`. Automatically mounts any external key directories outside `~/.ssh` into the tunnel container as read-only (`:ro`) with dual-path mapping (host absolute path and container `/root` path), eliminating missing identity errors without hardcoded paths or host permission pollution (`internal/tunnel/container.go`, `internal/tunnel/container_mount_test.go`, `docs/specs/tunnel.md`).

## [v1.14.4] - 2026-10-24

### Fixed
- **Tunnel GCP Config Placeholder Overwrite & Env Propagation**: Used `cp -rf` to force overwrite base image placeholder configurations (`config_default`) with host active configurations (`project`, `account`, dotfiles) when staging `~/.config/gcloud` into the tunnel container. Added host environment variable propagation (`CLOUDSDK_CORE_PROJECT`, `CLOUDSDK_ACTIVE_CONFIG_NAME`, `CLOUDSDK_COMPUTE_ZONE`, `CLOUDSDK_COMPUTE_REGION`), resolving `The required property [project] is not currently set` errors during IAP tunnel startup (`internal/tunnel/container.go`, `internal/tunnel/tunnel_test.go`, `docs/specs/tunnel.md`).

## [v1.14.3] - 2026-10-24

### Fixed
- **Tunnel GCP IAP Writable Staging (`~/.config/gcloud`)**: Mounted host `~/.config/gcloud` to read-only staging `/root/.config/gcloud-host` and copied to a local writable `/root/.config/gcloud` inside the tunnel container at startup. Added `CLOUDSDK_CORE_DISABLE_FILE_LOGGING=true`, preventing `[Errno 30] Read-only file system` errors on `credentials.db` during Google Cloud IAP tunnel establishment (`internal/tunnel/container.go`, `internal/tunnel/tunnel_test.go`, `docs/specs/tunnel.md`).

## [v1.14.2] - 2026-10-24

### Fixed
- **Tunnel SSH macOS Keychain Sanitization**: Added `-o IgnoreUnknown=UseKeychain` and automated `/root/.ssh/config` sanitization inside the tunnel container bootstrap script to prevent Alpine Linux OpenSSH crashes when host `~/.ssh/config` includes macOS-specific `UseKeychain yes` directives (`internal/tunnel/container.go`, `internal/tunnel/tunnel_test.go`, `docs/specs/tunnel.md`).

## [v1.14.1] - 2026-10-24

### Fixed
- **Docker Context Host Auto-Detection**: Centralized Go Docker SDK client initialization through `internal/docker.NewClient()`. Dynamically resolves the active Docker daemon endpoint via `docker context inspect --format '{{.Endpoints.docker.Host}}'` when `$DOCKER_HOST` is unset, preventing connection failures to `/var/run/docker.sock` on OrbStack, Colima, or custom Docker contexts (`internal/docker/host.go`, `cmd/tunnel.go`, `cmd/dns.go`, `cmd/ps.go`, `cmd/ip.go`, `cmd/logs.go`, `cmd/server.go`, `docs/specs/cli.md`).

## [v1.14.0] - 2026-10-23

### Added
- **Automatic `OOPS_IP` Environment Synchronization**: Auto-detects the bridge IP of the running `oops` container (preferring the `oops_edge` network) and synchronizes `OOPS_IP=<ip>` into `.env` and `ComposeCommandEnv()`. Refreshed immediately after the `edge` stack starts during `oops up` and when `oops server` initializes, enabling downstream services (such as forward proxies and test containers) to reference `${OOPS_IP}` in their `dns:` configuration to resolve internal `.oops` services without manual IP maintenance (`internal/docker/ip.go`, `internal/orchestrator/updater.go`, `cmd/server.go`, `docs/specs/oopsbox.md`).

## [v1.13.0] - 2026-10-23

### Added
- **Database Tunneling & Dynamic Service Discovery (`oops tunnel`)**: Implemented zero-host-port database tunneling via standard OpenSSH (with transparent GCP IAP support via `~/.ssh/config` ProxyCommand or direct SSH). Features Dynamic Service-to-IP and Exposed Port resolution on remote Docker bridge networks (querying container IP and ports for `mysql`, `redis`, etc.), Client Tunnel Container isolation on local Oops network (`google/cloud-sdk:alpine`), and direct DNS addressing via `<service>.<remote>.tunnel.oops:<port>` without host port mappings or localhost conflicts (`cmd/tunnel.go`, `internal/tunnel/`, `docs/specs/tunnel.md`).
- **Remote Manifest Tunnel Configuration (`tunnel.services`)**: Extended `remote.Config` in `internal/remote/config.go` with optional `tunnel.services` list declared inside `/@<remote>/remote.yaml` for zero-arg tunnel startup (`oops tunnel @<remote>`).
- **Multi-Hostname & Explicit DNS Mappings in Oops DNS**: Enhanced `internal/dns/watcher.go` to support multiple comma-separated domain names in `oops.dns.hostname` and custom per-service IP mappings via `oops.dns.mappings` for automatic resolution of tunnel endpoints with port collision support (secondary IP aliasing via `NET_ADMIN`).

## [v1.12.0] - 2026-10-23

### Added
- **Direct `dump.rdb` Support & Ergonomic Restore Shortcuts**: Extended `oops restore` to directly accept raw `dump.rdb` snapshots and engine shortcuts (e.g. `oops restore redis [dump.rdb]`, `oops restore dump.rdb`, `oops restore-db redis dump.rdb`). `FindLatestDBBackup` automatically discovers candidate `dump.rdb` files across workspace, backup directory, and `./data/redis/` (`cmd/restore.go`, `internal/backup/restore.go`).
- **Redis ACL & Key Prefix Namespace Management (`oops db redis`)**: Added `redis` engine support to `oops db` (`create`, `readonly`, `passwd`, `list`, `drop`). Provisions isolated Redis 7 ACL users with key prefix boundaries (`~<name>:*`), executes `ACL SAVE` for persistence, and exports credentials to `config/env/redis.<user>.env` with `REDIS_URL` (`cmd/db.go`, `internal/db/redis.go`).
- **Modernized Redis Stack Template**: Upgraded `oopsbox/compose.db.yaml` with production-grade tuning (AOF `appendfsync everysec`, `maxmemory 384mb`, `maxmemory-policy noeviction`, `maxclients 1000`, `timeout 1800`, Timezone `Asia/Bangkok`) and Docker Compose `pre_start` hook initializing `/data/users.acl` with default user credentials before boot.

### Fixed
- **Redis Restore with AOF Transpilation & Safety Backup**: Resolved issue where restoring a `dump.rdb` snapshot failed or resulted in 0 keys when `appendonly yes` was active. `oops restore` now safely backs up existing `/data/appendonlydir` and `/data/dump.rdb` to `.bak.<timestamp>`, stops the main container to prevent memory overwrite, and uses an ephemeral helper container to load RDB and dynamically generate the modern Redis 7 AOF structure (`CONFIG SET appendonly yes`) before restarting the main container (`internal/backup/restore.go`).

## [v1.11.0] - 2026-10-22

### Added
- **Dynamic UID/GID Subprocess Injection & Automatic `.env` Synchronization**: Auto-detects caller's `os.Getuid()` and `os.Getgid()`, injecting `UID` and `GID` into `ComposeCommandEnv()`, and automatically updates or appends them in workspace `.env` before invoking `docker compose`. This ensures containers configured with `user: "${UID:-1000}:${GID:-1000}"` execute with the exact caller credentials across both `oops` and direct `docker compose` invocations, preventing permission mismatch when deploying `.env` across disparate OS hosts (`internal/orchestrator/updater.go`).

## [v1.10.1] - 2026-10-21

### Fixed
- **Resolve `extends:` Images and Ports in `oops ps -a`**: Traversed `extends:` hierarchy (both string and object forms pointing to relative files or local service anchors) to correctly display inherited container images and port mappings for uncreated services (`cmd/ps.go`, `internal/docker/compose.go`).

## [v1.10.0] - 2026-10-21

### Changed
- **Consolidated `oops ps` & Deprecated `status`**: Retired `oops status` command in favor of `oops ps` as the single unified inspection command (`cmd/ps.go`, `cmd/root.go`).
- **Separated `STACK` & `SERVICE` Table Columns**: Refactored output table to display `STACK` and `SERVICE` as separate distinct columns instead of concatenated `STACK/SERVICE` for improved readability (`cmd/ps.go`).

### Added
- **`oops ps -a` (`--all`) Missing Services Detection**: Added `-a` / `--all` flag to `oops ps` that discovers all compose service definitions across stacks and displays uncreated services with status `missing` (`cmd/ps.go`).

## [v1.9.1] - 2026-10-18

### Added
- **Shared Compose Extension (`oopsbox/common.yaml`)**: Extracted shared `common-logging` configuration (`driver: "json-file"`, `max-size: "10m"`, `max-file: "3"`) into `common.yaml` and `.example`, eliminating duplicate `x-logging` anchor definitions across all stack compose files via `extends: { file: common.yaml, service: common-logging }`.

### Fixed
- **CLI Workspace & Path Resolution Outside Oopsbox Directory**:
  - `oops db`: Resolved `workDir` and `ValidateActiveBox` so credential export files are written to `<workDir>/config/env/` instead of the caller's working directory (`cmd/db.go`).
  - `oops backup` / `restore`: Loaded `oops.yaml` from resolved `workDir` and anchored relative `backupDir` under `<workDir>/backups` (`cmd/backup.go`, `cmd/restore.go`).
  - `oops volume check`: Anchored `backupDir` to `<workDir>/backups` (`cmd/volume.go`).
  - `oops remote` / `deploy`: Auto-resolved active box in `RequireOopsboxWorkspace` when invoked without `-C` outside the workspace (`cmd/remote.go`).
  - `oops down` / `box stop`: Built full `ComposeFiles` `-f` chains during stack teardown (`cmd/down.go`, `cmd/box.go`).
  - Database container dump/restore: Added fallback to container environment for root/database passwords when env vars are unset in the host shell (`internal/backup/backup.go`, `internal/backup/restore.go`).

## [v1.9.0] - 2026-10-18

### Added
- **Enhanced `oops ps` (`oops status`) Table Columns & `--full` (`-fl`) Flag**: Added `CONTAINER ID`, unified `STACK/SERVICE` (`/<stack>/<service>` resolved from `compose.<stack>.yaml`), `IMAGE` (shortened `<image>:<tag>` by default, full registry/repository path with `--full` or `-fl`), and `STATUS` with uptime and health (`cmd/status.go`, `cmd/root.go`).

### Fixed
- **CLI Table Column Alignment**: Removed un-tabbed dashed separator lines inside `text/tabwriter` across `oops ps` (`cmd/status.go`), `oops ips` (`cmd/ip.go`), and `oops dns` (`internal/dns/inspector.go`) so header columns align with data rows like `docker ps`.

## [v1.8.1] - 2026-10-18

### Fixed
- Suppressed false-positive `Found orphan containers (...) for this project` warnings during `oops up`, `oops update`, and webhook recreation by injecting `COMPOSE_IGNORE_ORPHANS=True` into `docker compose` execution environments (`internal/orchestrator/updater.go`, `internal/docker/client.go`).
- Fixed `oops pull` and webhook container recreation/restart passing only the root `compose.yaml` anchor instead of the full `ComposeFiles` `-f` chain (`cmd/pull.go`, `internal/docker/client.go`).

## [v1.8.0] - 2026-10-17

### Added
- **Default Container Log Rotation (`x-logging: &default-logging`)**: Added `json-file` log rotation (`max-size: "10m"`, `max-file: "3"`) across `oopsbox/compose.yaml`, `oopsbox/compose.yaml.example`, `internal/remote/blueprint.go`, and all Oopsbox stack manifests (`compose.edge.yaml`, `compose.db.yaml`, `compose.storage.yaml`, `compose.tools.yaml`) to prevent unbounded container logs from exhausting host disk space.

## [v1.7.1] - 2026-10-17

### Fixed
- Fixed `oops logs` failing with `no such service: caddy-proxy` by passing the full deduplicated `ComposeFiles` `-f` chain (instead of only the root `compose.yaml` anchor) to `docker compose logs`, and bound `Stdout`/`Stderr` on fallback `docker logs` calls (`cmd/logs.go`).

## [v1.7.0] - 2026-10-17

### Added
- **`x-oops-stack` Ordering & Selective Auto-Start**: Added `x-oops-stack:` support in both `compose.yaml` and `compose.<stack>.yaml` (`internal/docker/compose.go`, `internal/docker/resolver.go`) to control exact top-to-bottom boot order and selective service auto-start on `oops up`. Services or stacks omitted from `x-oops-stack:` do not auto-start unless explicitly targeted.
- **Pre-Flight `depends_on` Warnings on Deploy**: Added pre-flight validation in `ResolveRequiredDependencies` (`internal/remote/resolve.go`, `cmd/remote.go`) to warn operators during `oops deploy` / `oops remote push` when a service declares `depends_on:` for a local or cross-stack service that is omitted from `x-oops-stack:`.

### Changed
- **Pure Stack Architecture**: Replaced `oopsbox/services/` and `x-oops-groups:` with self-contained stack manifests (`oopsbox/compose.edge.yaml`, `oopsbox/compose.db.yaml`, `oopsbox/compose.storage.yaml`, `oopsbox/compose.tools.yaml`) that declare their own stack-scoped bridge networks (`oops_edge`, `oops_db`, `oops_storage`), while `compose.yaml` defines the default untrusted sandbox network (`oops_default`).
- **Pure Configuration `oops.yaml`**: Moved the `oops` daemon container definition into `compose.edge.yaml` so `oops.yaml` and `oops.yaml.example` act strictly as pure `x-oops:` system configuration files and are no longer chained via `-f` into `docker compose`.
- **MySQL LTS Upgrade**: Upgraded default MySQL image in `oopsbox/compose.db.yaml` to `mysql:8.4` (LTS).

## [v1.6.0] - 2026-10-17

### Added
- **Capsule-Scoped Default Delivery (Omitted Stack Invariant)**: In `oops deploy @remote` and `oops remote push @remote`, omitting `/stack` restricts discovery and deployment strictly to manifests, stacks, and services declared within the target capsule `/@<remote>/`. Root workspace stacks that do not belong to the capsule are excluded from streaming.

## [v1.5.0] - 2026-10-17

### Added
- Introduced `oops remote repair [@<remote>]` (alias: `oops remote doctor`) to audit and repair directory structures and missing blueprint files across local capsules and remote hosts.
- Enforced Self-Healing Workspace Invariant across remote operations (`push`, `pull`, `connect`, `deploy`), ensuring standard Oopsbox directories (`services/`, `config/`, `data/`, `backups/`) and core blueprint manifests exist prior to file streaming.

## [v1.4.0] - 2026-10-17

### Added
- Automated Oopsbox directory structure provisioning (`services/`, `config/`, `data/`, `backups/`) and initial workspace delivery during `oops remote connect @<remote>` for fresh remote servers.
- Automated `.env` generation with strong cryptographic credentials (`OOPS_SECRET` and `OOPS_DB_*_PASSWORD`) and `config/` directory initialization during capsule blueprint setup.

## [v1.3.1] - 2026-10-17

### Fixed
- Suppressed macOS extended attributes (`--no-xattrs`, `--no-mac-metadata`, `COPYFILE_DISABLE=1`) during `oops remote push` and `oops deploy` tar streaming to eliminate `LIBARCHIVE.xattr.com.apple.provenance` warnings on Linux servers.
- Resolved `x-oops-groups` dependencies and inner service names (`services/*.yaml`) during remote deployment dependency resolution (`ResolveRequiredDependencies`).

## [v1.3.0] - 2026-10-16

### Added
- Standardized declarative infrastructure configuration in `oops.yaml` under `x-oops:` (`secret`, `db`, `orchestrator`, `backup`, `dns`, `volume`, `registries`).
- Environment Variable Parity with `x-oops = OOPS_` tree rule: automatically loads project defaults from `oops.yaml` with shell / `.env` variable overrides taking precedence.
- Dedicated namespace isolation for database credentials under `OOPS_DB_*` (`OOPS_DB_MYSQL_PASSWORD`, `OOPS_DB_POSTGRES_PASSWORD`, `OOPS_DB_REDIS_PASSWORD`) preventing environment variable collisions with container applications.
- Cross-stack dependency and service resolution supporting `/stack`, `/<stack>/<service>`, and direct `services/<name>` templates with local definition precedence.
- Enforced Local Service Precedence in target resolution: project services defined inside `compose.<stack>.yaml` shadow matching shared templates in `services/`, preventing template injection when dependencies match local definitions.
- Extended Local & Remote Precedence to `oops remote push` and `oops deploy`: dependencies declared locally in stack manifests or within `@remote` capsules strictly suppress pulling conflicting default templates from workspace `services/` and `config/`.
- Multi-stack target support in `oops remote push`, `oops deploy`, and `oops remote pull` using comma-separated notation (`/<stack1>,/<stack2>`), treating target stacks as an explicit delivery cluster.
- Strict Pre-Flight Dependency Guard for remote push/deploy: automatically validates cross-stack dependencies prior to file transmission; aborts deployment if referenced stacks are neither included in push targets nor provided within the remote capsule scope, preventing accidental leaks of local workspace stacks to remote environments.

### Changed
- Converted `oops.yaml` to a standard Docker Compose manifest declaring `services: oops` and `x-oops:` settings block.
- Streamlined `oopsbox/.env.example` to focus strictly on secrets (`OOPS_SECRET`, database passwords) and documented optional overrides.

## [v1.2.0] - 2026-10-14

### Added
- Symmetric Git/Remote Model: Introduced `oops remote push [/<stack>] [@<remote>]` and `oops remote pull [/<stack>] [@<remote>]` for bidirectional capsule synchronization.
- `.oopsallow` default-deny whitelist engine for bidirectional synchronization (`push` and `pull`), protecting local capsules from downloading large database/backup files (`data/`, `backups/`).
- Blueprint-driven initialization during `oops remote connect`: automatically creates `compose.yaml`, `oops.yaml`, and `.oopsallow` from `.example` blueprints when connecting to a fresh host.
- Services Dependency Resolution Algorithm in `oops remote push` / `oops deploy`: selectively packages required service templates (`services/<name>.yaml`) and configs (`config/<name>/`) by scanning `x-oops-services:`, container `depends_on:`, and remote local overrides.
- Blueprint templates in Oopsbox workspace: `compose.yaml.example`, `oops.yaml.example`, and `.oopsallow.example`.
- SCP-style colon remote path parsing in `oops remote connect @<remote>[:<path>] [remote-path] [--path <p>]`, supporting home-relative paths (`@prod:path`, `@prod:~/path`) and absolute paths (`@prod:/opt/path`).
- `oops remote setup [@<remote>]` command to unconditionally install or upgrade the remote server to the latest `oops` CLI binary from GitHub releases and ensure `docker-compose-plugin` is present.
- Automatic server prerequisite checking (`docker compose` and `oops` binary) and setup bootstrap during `oops remote connect`.
- Dynamic remote path resolution in `oops rx` from `remote.yaml` configuration.

### Changed
- Standardized `oops deploy` as an exact ergonomic alias for `oops remote push` with pure delivery semantics (no automatic container startup; outputs post-delivery execution hint for `oops rx`).
- Clarified `oops rx [@<remote>] <oops-cmd> [args...]` as forwarding exclusively `oops` subcommands inside the remote Oopsbox workspace.
- Renamed `oops remote add` to `oops remote connect @<remote> [remote-path]`.
- Standardized `oops remote remove @<remote>` to use canonical `@<remote>` syntax across CLI and documentation.
- Standardized CLI stack syntax strictly on `/<stack>` (`compose.<stack>.yaml`) and eliminated `/<project>` notation across the entire project.

### Removed
- Removed legacy standalone `oops sync` command hierarchy (`oops sync push|pull|env|secret`) in favor of unified `oops remote push` and `oops remote pull` governed by `.oopsallow`.
- Removed automatic `docker compose up -d` execution from `oops deploy`.

## [v1.1.0] - 2026-10-09

### Added
- Dedicated `oops db <engine> readonly <db_name> <app_name>` subcommand to explicitly provision SELECT-only database credentials mapping to `<db_name>__ro_<app_name>`.
- Automated `.env` file generation specifically scoped to readonly user credentials.
- MySQL implicit wildcard inheritance for Top User granting `xxx\_%` upon Level 1 creation.
- PostgreSQL explicit inheritance mapping Level 2 databases to Level 1 Top User via `GRANT`.

### Changed
- Refactored `oops db <engine> create` syntax to accept a single `<db_name>`, enforcing a strict 1:1 database-to-user architecture.
- Enforced maximum 2-level hierarchy naming constraint (max one underscore `_`) for database provisioning.

## [v1.0.0] - 2026-10-09

### Added
- Added SeaweedFS object storage configuration (`compose.storage.yml`) to support S3-compatible workloads locally.
- Introduced `x-oops-depends_on` inside compose manifests for automated topological dependency resolution, replacing the central `profiles:` block in `oops.yml`.

### Changed
- `oops up` now targets the default `/.` stack (`compose.yml`) instead of `@default`, enabling fully decentralized project-level architecture.
- Reorganized edge configuration files (`compose.edge-caddy.yml`, etc.) and updated orchestration paths.

### Removed
- Abolished `@profile` syntax and legacy `@all` dynamic resolver support.

## [v0.18.3] - 2026-10-08

### Changed
- Simplified stack discovery by replacing the directory-based topology (`stacks/<name>/compose.yml`) with a flat-file topology (`compose.<name>.yml` at the project root).

## [v0.18.2] - 2026-10-08

### Changed
- Moved `dns.tld` configuration from the workstation-specific `oopsbox.yml` to the shared project manifest `oops.yml` to enforce a unified top-level domain across the team.

## [v0.18.1] - 2026-10-08

### Fixed
- Atomic Self-Update on Linux: Resolved persistent `Text file busy` (`ETXTBSY`) errors during `oops selfupdate` when `mv` falls back to `cp` across filesystems, by explicitly executing `sudo rm -f` before moving the replacement binary. Also fixed the sudo fallback error hint to display the binary's absolute path (e.g., `sudo /var/lib/google/bin/oops selfupdate`) to avoid `command not found` when `sudo` lacks the binary in `secure_path`.

## [v0.18.0] - 2026-10-08

### Added
- Remote Workspace Cloning: Added `oops box clone <ssh-target> [-r <remote>] [path]` command to clone an Oopsbox workspace from a remote server over SSH without GitHub/GitLab, automatically registering the local Git remote for `oops deploy`.

## [v0.17.2] - 2026-10-08

### Fixed
- Prepend PATH in COS Shell Profiles: Prepend `export PATH="/var/lib/google/bin:$PATH"` to line 1 of shell profiles (`.bashrc`, `.bash_profile`, `.profile`) during COS bootstrap so non-interactive SSH shells evaluate PATH before early `return` blocks.

## [v0.17.1] - 2026-10-08

### Changed
- Renamed `oops box create <path>` to `oops box init [path]`, making `<path>` optional (defaulting to current directory `.`).

### Fixed
- Atomic Self-Update on Linux: Replaced file overwrite (`cp`) with atomic move (`mv -f`) in `oops selfupdate` when escalating to `sudo`, avoiding `Text file busy` (`ETXTBSY`) errors on Linux.
- Remote Execution & COS Shell PATH: Updated `oops rx` SSH wrapper to check `command -v oops` silently before falling back to `/var/lib/google/bin/oops`, preventing `bash: line 1: oops: command not found` noise on non-interactive SSH connections. Automatically append `/var/lib/google/bin` to user shell profiles (`.bashrc`, `.bash_profile`, `.profile`) during COS bootstrap.

## [v0.17.0] - 2026-10-08

### Added
- Remote Server Rename: Added `oops remote rename <old-name> <new-name>` subcommand to rename registered remote server configurations.
- Git Remote Prefix Namespacing: Namespaced local Git remotes with `oops-` prefix (e.g. `oops-prod`) while exposing clean logical names (`prod`) in CLI commands.
- Redesigned `oops remote add <ssh-target> [-r <name>]`: Positional `<ssh-target>` parsing with `-r / --remote` option flag, cascading `prod` default, `<host>` / `<host>_<user>` fallback, and interactive conflict prompts.
- Default Remote Target Alignment: Standardized `oops deploy` and `oops rx` default target resolution to `prod` (mapped to `oops-prod`).
- Remote Server Management & Git Bare Sync: Added `oops remote` CLI command suite (`add`, `list`, `remove`, `push`, `deploy`, `pull`, `<cmd>`) for 1-command remote server bootstrapping via SSH (`~/.ssh/config` alias/IAP), verifying and installing `oops` CLI and `docker-compose-plugin`.
- Google Container-Optimized OS (COS) Support: Directs binary installations to `/var/lib/google/bin/oops` and `/var/lib/google/docker-cli-plugins/docker-compose`, updating `~/.docker/config.json` with `"cliPluginsExtraDirs": ["/var/lib/google/docker-cli-plugins"]`.
- Server-side Git Bare Init: Added `oops box init-bare <bare-path> <oopsbox-path>` command and post-receive hook generator supporting explicit `-o deploy` gate (`git push -o deploy <server> <ref>`), multi-ref/multi-tag push handling, and existing workspace seeding.
- Living Specification: Created [docs/specs/remote.md](docs/specs/remote.md) detailing the `oops remote` architecture.

## [v0.16.0] - 2026-10-06

### Added
- Deploy Key Management: Added `oops key`, `oops key reset`, and `oops key set` commands for automated host-level `ed25519` SSH Deploy Key generation and configuration.
- Workspace Cloning: Added `oops box clone <repo> [path]` using local Deploy Keys, featuring automatic authentication failure detection with direct links to add keys on GitHub/GitLab.

## [0.15.0] - 2026-10-06

### Added
- Storage Guard: `oops up`, `restart`, `update`, `switch` and `box start` inspect every bind-mount source of the services they touch. Services using a path through a dead link, or a path under a configured mount prefix that is still on the OS disk, are blocked together with lower-priority services; unaffected services (edge first) still run and the command exits non-zero. Configure via `storage.mount_prefixes` and `priority` in `oops.yml`. See `docs/specs/storage-guard.md`.
- Backup commands (`backup*`, `restore*`) refuse to run when `OOPS_BACKUP_DIR` is a dead link or sits on the OS disk under a mount prefix, independently of container management.
- `oops storage check` reports dead links and unmounted paths; `oops storage link <target>` links the oopsbox `data` directory to a persistent disk after validation.

### Documentation
- Added `docs/architecture/data-persistent-storage.md` describing persistent data and storage path abstraction best practices, including host-local backups and why whole-disk snapshots are discouraged.

### Changed
- `oops up` now reuses the orchestrator start logic instead of a duplicated inline implementation.

### Fixed
- Edge stack: oops ingress upstream and healthcheck now use the default port `8080`.
- `install.sh` configures `PATH` and completion in shell profiles and sets `chmod 755` on the install directory and binary.

## [0.14.2] - 2026-10-06

### Fixed
- Fixed permission error (`Permission denied`) in `install.sh` on Google Container-Optimized OS (COS) when installing to `/var/lib/google/bin` by dynamically detecting directory write permissions and requesting `sudo` as needed.

## [0.14.1] - 2026-10-05

### Fixed
- Removed redundant `oops ips` subcommand; `oops ip` lists all container IP addresses when invoked without parameters.

## [0.14.0] - 2026-10-05

### Features
- Implemented container status inspection command (`oops status`) with Docker Engine API integration showing status, health, IP, and formatted ports.
- Refined port formatting rules in `oops status` with `*:` (all-interface `0.0.0.0`/`::`), `#:` (loopback `127.0.0.1`), multi-port array grouping (`*:[9001,9002]->9000`), port deduplication, and `PORTS (*=all, #=local)` table header legend.
- Set default server listening port to `8080` for non-root safety and edge proxy compatibility.

### Changed
- Streamlined CLI subcommands by removing legacy aliases (`webhook`, `daemon`, `db-backup`, `data-backup`, `db-restore`, `data-restore`, `delete`, `rm`).
- Updated `oops db` help and living specifications to explicitly list supported database engines (`mysql`, `pg`/`postgres`).
- Updated Jarn framework to v0.10.1 and declared `Release Mode: host-release` in `AGENTS.md`.

### Fixed
- Release workflow no longer fails with `release not found` when tag is pushed before the release object is created.

## [0.13.0] - 2026-10-05

### Added
- Native self-update command `oops selfupdate` with `--check` (`-c`) and `--force` (`-f`) flags to upgrade binary directly from GitHub releases.
- Version discovery commands and flags (`oops version`, `oops -v`, `oops --version`) using Go runtime `debug.ReadBuildInfo()` and CI `-ldflags` SemVer injection.
- Container IP lookup command `oops ip <service>` to query network IP addresses by service name, container name, or domain.
- Global container IP table command `oops ips` listing all active Docker container IP addresses sorted numerically by IP.
- Full log streaming command `oops logs [targets...]` supporting single containers, sub-stacks (`/edge`, `/db`), multi-services, wildcards (`app..`), default tail of 50 lines, and follow mode (`-f`).
- Dynamic Docker bridge subnet discovery (`DiscoverDockerSubnets`) and Colima VM NAT routing configuration (`SetupColimaRouting`).

### Changed
- Sorted DNS inspection table (`oops dns list`) numerically by IP address instead of lexicographical hostname.
- Updated default `oops logs` tail length to 50 lines (approx. 1 terminal screen height).

### Fixed
- Replaced hardcoded macOS gateway IPs with dynamic Docker container gateway inspection (`GetHostGatewayIP`).
- Resolved `/etc/resolver/<tld>` target to the active `oops` container IP address (`GetOopsContainerIP`).
- Fixed compose variable interpolation errors during `oops logs` by automatically detecting and passing workspace `.env` file.

### Added
- Native Go `oops box` CLI subcommand suite (`create`, `active`, `list`, `start`, `stop`, `switch`, `cert`) for complete workspace lifecycle management without external bash wrappers.
- Dynamic blueprint download and extraction from latest GitHub Release asset `oopsbox.tar.gz` (with fallback to `main` branch archive) on `oops box create <path>`.
- Cryptographic credential generation for `OOPS_SECRET` (48 chars) and shared database passwords (32 chars) upon workspace creation.
- Built-in shell tab auto-completion for Zsh and Bash (`oops completion`) with automatic configuration in `install.sh`.
- Google Container-Optimized OS (COS) auto-detection in `install.sh` targeting `/var/lib/google/bin/`.
- Redis password authentication and container healthcheck support.

### Changed
- Converted `install.sh` to a standalone Oops CLI binary installer with Google Container-Optimized OS (COS) support and shell completion setup.
- Synchronized living specifications in `docs/specs/oopsbox.md` and `docs/specs/cli.md`.

### Fixed
- Auto-detected and injected workspace root `.env` via `--env-file` during `docker compose` execution across sub-stacks.
- Resolved `host.oops` to container-accessible Docker bridge gateway IP (`192.168.215.1`) instead of loopback.

### Removed
- Removed legacy `oopsbox/bin/` workstation shell scripts (`oopsbox`, `oopsbox-mac`, `oopsbox-linux`) in favor of native Go `oops box` CLI.
- Removed redundant `install-cli.sh` in favor of unified standalone installer `install.sh`.

## [0.11.0] - 2026-10-05

### Added
- Flexible project directory target support in `install.sh` (`curl ... | bash -s -- <project-directory>`) with seamless default installation into the current directory (`$(pwd)`).

### Changed
- Made `oops` root command explicitly display the Help menu by default upon execution without subcommands.
- Updated `install.sh` to use clean, readable Bash idioms and unified `README.md` quickstart installation guides.

### Removed
- Removed legacy single-container `docker-compose.yml` from repository root in favor of modular multi-stack manifests under `oopsbox/stacks/`.

## [0.10.0] - 2026-10-05

### Added
- Standalone 1-line CLI installer script (`install-cli.sh`) supporting auto-detection of operating system (`darwin`, `linux`) and architecture (`arm64`, `amd64`) with customizable installation destination.
- Native standalone `oops` CLI binary installation as primary in `install.sh`, `oopsbox-mac`, and `oopsbox-linux` with automatic PATH symlinking and Docker wrapper fallback.

### Changed
- Streamlined Quickstart documentation in `README.md` to prioritize unified 1-line installation workflows.

## [0.9.0] - 2026-10-05

### Added
- Multi-OS raw binary releases (`oops-linux-amd64`, `oops-linux-arm64`, `oops-darwin-amd64`, `oops-darwin-arm64`) with SHA-256 `checksums.txt` compiled and published via GitHub Actions.
- Oopsbox blueprint package asset (`oopsbox.tar.gz`) generated and attached to GitHub Releases for fast, pinned-version provisioning.
- In-place workstation upgrade command (`oopsbox upgrade [tag]`, alias: `update`) in `oopsbox-mac` and `oopsbox-linux` to safely update `bin/` scripts and templates without touching user data or custom `.env`.

### Changed
- Optimized `install.sh` one-line installer to prioritize downloading release `oopsbox.tar.gz` directly with graceful fallback to `main` branch archive.

### Fixed
- Handled `scanner.Err()` checks across backup manifest and DNS records parser, and standardized test contexts using `t.Context()`.

## [0.8.0] - 2026-10-05

### Added
- Single-source Active Box pointer (`~/.oops/active_box`) recorded automatically on `oopsbox start`, `oopsbox switch`, and `oopsbox install`.
- Active Box Mismatch Guard (`ValidateActiveBox`) in `oops` CLI to prevent accidental execution across different Oopsbox repositories on the same workstation.
- `--wipe-all` flag on `oops down` with interactive confirmation (and `-y, --yes` bypass) to forcefully stop and remove all containers on the Docker daemon.
- Seamless multi-box handover via `oopsbox switch <path>` to stop the current active box and initialize the new target box cleanly.

### Changed
- Renamed workstation CLI and driver scripts from `devoops` to `oopsbox` (`bin/oopsbox`, `bin/oopsbox-mac`, `bin/oopsbox-linux`).
- Renamed workstation configuration template from `devoops.yml.example` to `oopsbox.yml.example` (and active config lookup to `oopsbox.yml`).
- Renamed living specification from `docs/specs/devoops.md` to `docs/specs/oopsbox.md`.
- Removed all legacy backward-compatibility fallbacks, obsolete struct fields (`Colima`, `Aliases`), and deprecated environment variables (`STOP_TIMEOUT_SECONDS`, `HEALTHCHECK_*_SECONDS`, `PORT`, `BACKUP_RETENTION_DAYS`, `BACKUP_DIR`, `OOPS_HOSTS_FILE`) for clean-break canonical parity.

## [0.7.0] - 2026-10-04

### Added
- One-line curl installer script (`install.sh`) for rapid provisioning of Oopsbox and global CLI wrapper setup (`curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash`).
- Primary `OOPSBOX_DIR` environment variable support in `ResolveWorkDir` with backward-compatible `OOPS_DIR` fallback.
- Docker CLI wrapper and shell function documentation for running `oops` from any directory or on Google Container-Optimized OS (COS) and locked-down `noexec` environments.


## [0.6.0] - 2026-10-04

### Added
- Dedicated development utilities group `stacks/tool/` with lightweight test services (`httpbin` and `mailpit`) connected to `net-edge`.
- Modular service configuration path convention (`stacks/<group>/<service>/`, e.g. `stacks/edge/caddy/Caddyfile`, `stacks/db/mysql/my.cnf`).

### Changed
- Consolidated `oops` service daemon directly into `stacks/edge/compose.yml` and removed legacy `stacks/utils/` directory.
- Standardized profile definitions under `profiles:` in master `oops.yml` (`default: [/edge]`) as the single source of truth, removing `OOPS_DEFAULT_GROUP`.
- Streamlined `devoops` workstation tooling (`bin/devoops`, `bin/devoops-mac`, `bin/devoops-linux`) to focus exclusively on workstation bootstrapping, network routing, and OS certificate trust while delegating all profile and stack orchestration to the `oops` CLI.
- Updated edge reverse proxy image tag to `lucaslorentz/caddy-docker-proxy:alpine`.

## [0.5.0] - 2026-10-04

### Added
- Native DNS management CLI (`oops dns` / `oops dns list`, `oops dns get <domain>`, `oops dns set <domain> <ip>`, `oops dns del <domain>`, `oops dns reload`) supporting `.wildcard` domain prefixes.
- Active container protection validations preventing `dns set` conflict or `dns del` removal on dynamic Docker service hostnames.
- Pure file-watcher and atomic safe hot-reload for static DNS records in `oops server` with zero downtime.
- Canonical local node dynamic DNS storage in `data/oops/dns.records`.
- Shared team / infra DNS configuration support (`dns.upstreams: [...]` and `dns.records: [...]`) directly in `oops.yml`.
- Dedicated workstation configuration file `devoops.yml` (and `devoops.yml.example`) at `oopsbox/` root separating local developer runtime/DNS settings from master `oops.yml` manifest.
- Multi-engine runtime configuration (`engine.type: auto | orbstack | colima | docker`) in `devoops.yml` with OrbStack native container routing support.
- Configurable DNS top-level domain (`dns.tld` in `devoops.yml` and `OOPS_DNS_TLD`, default: `oops`) for dynamic macOS `/etc/resolver/<tld>` binding.
- Healthcheck configuration for `oops` service daemon in `stacks/utils/compose.yml`.

### Changed
- Promoted `templates/oopsbox/` to repo root `oopsbox/` blueprint directory.
- Moved master stack manifest `stacks/oops.yml` to root `oopsbox/oops.yml`.
- Standardized `devoops` DNS and static host provisioning to `data/oops/dns.records`.
- Cleaned up command aliases to maintain strict CLI canonical discipline.

## [0.4.0] - 2026-10-04

### Added
- Configurable Colima VM resource settings (`colima:` with `cpu`, `memory`, `disk`, and `vm_type`) in `stacks/oops.yml` and `OopsConfig`.
- Automated static hosts generation in `config/oops/hosts` mapping `host.oops` (macOS workstation gateway) and `vm.oops` (Colima VM / Linux Docker engine).
- Persistent VM firewall auto-repair background daemon (`fw-watcher.sh`) in Colima listening on Docker start events.
- Dynamic macOS DNS resolver binding directly to the `oops` DNS container IP upon `devoops start`.

### Changed
- Removed redundant `backup_data` named volume in `stacks/utils/compose.yml` to ensure backups write directly to the physical host filesystem (`./backups`).
- Set explicit container entrypoint and command (`entrypoint: ["oops"]`, `command: ["server"]`) in `stacks/utils/compose.yml` for backward/forward image compatibility.

## [0.3.0] - 2026-10-04

### Added
- Standardized workstation developer tooling suite `devoops` under `templates/oopsbox/bin/` (`bin/devoops`, `bin/devoops-mac`, `bin/devoops-linux`) with OS auto-dispatching and companion spec `docs/specs/devoops.md`.
- Unified master configuration file `stacks/oops.yml` consolidating registry aliases (`registries:`) and service groups (`groups:`).
- Image-based sequential rolling update and restart engine (`oops update --image <image/alias>`, `oops restart -i <image/alias>`).
- Registry alias resolution engine expanding shorthands (e.g. `gar/app:v1.0` -> `asia-southeast1-docker.pkg.dev/.../app:v1.0`).
- Multi-stack image pulling engine (`oops pull`, `oops pull --all`, `oops pull @<group>`, `oops pull <alias>/<image>`).
- Unified automated backup suite (`oops backup`, `oops backup-db`, `oops backup-data`) with dedicated prune subcommands and Redis RDB snapshot support.
- Configurable filesystem and volume data backup engine (`backups.data` in `stacks/oops.yml`) supporting named tar.gz archives.
- Safe interactive restore engine (`oops restore`, `oops restore-db`, `oops restore-data`, `oops backup inspect`) with embedded metadata manifests (`.oops-backup.json` and SQL comment headers), database confirmation typing guards, workspace validation, dry-run previews, and graceful fallback with notices for non-metadata external archives.
- Profile switching engine (`oops switch <target>`) that starts the target group/stack and stops all other services.
- Stop exclusion flags (`oops stop -x, --except, --exclude <target>`) to stop all services except specified exclusions.

## [0.2.0] - 2026-10-04

### Added
- Unified DevOps orchestration CLI subcommands (`server`, `up`, `stop`, `restart`, `down`, `status`, `logs`, `pull`, `update`, `db`, `db-backup`).
- Multi-stack compose resolution supporting `stacks/` directory structure, slash stack prefix (`/<stack>`, `/<stack>/<service>`), and double-dot wildcard matching (`app..`, `..worker`, `..api..`).
- Physical storage partitioning with dedicated `data/` (live realtime NVMe storage) and `backups/` (cold dumps).
- Sequential lifecycle pre-stop hooks (`oops.stop.cmd`, `oops.stop.timeout`) and inter-service delay support (`-d`, `--delay`).
- Database provisioning engine (`oops db <engine>[:<target>] <action>`) with secure random 20-character password generation.
- Automated database dump and retention pruning manager (`oops db-backup [targets...] [-r 7d]`).
- Synchronized Webhook deployment daemon with unified target selector (`target: "app.."`) and delay support.
- Native embedded event-driven DNS discovery daemon (`internal/dns/`) resolving wildcard hostnames (`.web.oops`), exact hostnames (`mail.test`), and upstream forwarding via `OOPS_DNS_UPSTREAM`.
- Custom static DNS hosts loader (`config/oops/hosts` & `OOPS_DNS_RECORDS`) supporting host machine/Colima IP mappings (`hostmac`, `host.docker.internal`).
- Version-controlled `config/` directory separation (`config/oops/hosts`, `config/caddy/`, `config/mysql/`) alongside `stacks/`, `data/`, and `backups/`.
- Working directory auto-discovery and path parity via `-C, --dir <path>`, `OOPS_DIR` environment variable, and fallback probing for `~/oopsbox` and `/opt/oopsbox`.
- Group Stacks configuration (`stacks/groups.yml`) supporting `@<group>` selector (e.g. `@core`, `@pg`, `@minimal`, `@all`) and `OOPS_DEFAULT_GROUP` startup fallback.
- Oopsbox local development tooling suite (`local-dev/oopsbox`, `local-dev/oopsbox-mac`, `local-dev/oopsbox-linux`) supporting one-command installer, Colima network routing, macOS DNS resolver, and macOS Keychain SSL certificate trust.
- Oopsbox blueprint template (`templates/oopsbox/`).

### Changed
- Refactored project topology and ubiquitous language from "Layer" to "Stack" (`stacks/edge`, `stacks/db`, `stacks/apps`, `stacks/utils`) and defined host deployment entity "Oopsbox".
- Standardized Oops engine environment variables with `OOPS_` prefix, explicit FQDN domain (`OOPS_DOMAIN`), and human-friendly duration strings (`OOPS_PORT`, `OOPS_STOP_TIMEOUT=30s`, `OOPS_HEALTHCHECK_TIMEOUT=10m`, `OOPS_HEALTHCHECK_INTERVAL=3s`, `OOPS_BACKUP_RETENTION=7d`, `OOPS_BACKUP_DIR=./backups`).

## [0.1.3] - 2026-09-19

### Maintenance
- Initialize Jarn Blueprint v0.3.1 and adapt repository standards (AGENTS, CONTRIBUTING, README).

## [0.1.0] - Initial

### Added
- Initial project scaffolding using Jarn Blueprint.
