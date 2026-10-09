# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project adheres to Semantic Versioning.

## [Unreleased]

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
