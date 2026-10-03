# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project adheres to Semantic Versioning.

## [Unreleased]

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
