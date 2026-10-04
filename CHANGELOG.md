# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project adheres to Semantic Versioning.

## [Unreleased]

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
