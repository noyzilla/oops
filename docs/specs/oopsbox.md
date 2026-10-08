---
title: Oopsbox Workstation Tooling Suite
status: active
tags: [oopsbox, local-dev, macos, linux, colima, orbstack, resolver, ssl-ca, cli]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/cli.md", "docs/specs/dns.md", "docs/specs/storage-guard.md"]
---

# Specification: Oopsbox Workstation Tooling Suite

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/cli.md](cli.md), [docs/specs/dns.md](dns.md), [docs/specs/storage-guard.md](storage-guard.md)

## Overview & Scope

`oopsbox` is the developer workstation orchestration suite. Integrated directly into the native Go CLI as `oops box <command>`, it manages local development environments on macOS and Linux desktop machines, abstracting VM and container engine provisioning (OrbStack / Colima / Docker Engine), host firewall packet filtering, local DNS resolver integration (`/etc/resolver/`), and root CA certificate trust for local TLS wildcard domains (`https://*.web.oops` or custom TLDs).

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Oops Box Manager (`oops box`)**: Native Go CLI command group for creating, activating, starting, switching, and securing Oopsbox workspaces.
- **Active Box State (`~/.oops/active_box`)**: System file recording the absolute path of the currently active Oopsbox workspace on the machine.
- **Release Blueprint (`oopsbox.tar.gz`)**: Tarball release asset containing the clean Oopsbox workspace structure (`stacks/`, `data/`, `oopsbox.yml.example`).
- **VM / Container Engine**: The underlying virtualization and container engine (`orbstack`, `colima`, or `docker`).
- **Local TLD Resolver**: macOS `/etc/resolver/<domain>` entry forwarding local domain resolution to the Oops embedded DNS watcher.
- **Local Root CA**: Caddy-generated internal root certificate installed into the host OS trust store or macOS Keychain.

## Business Rules & Logic Invariants

### Workspace Creation & Blueprint Download (`oops box init [path]`)
When provisioning a new workspace via `oops box init [path]`:
- **Target Directory Setup**: Expands `~` and relative paths to absolute canonical path, creating target directory if it does not exist.
- **Dynamic Blueprint Fetch**: Downloads the latest official release blueprint `oopsbox.tar.gz` from GitHub Releases (`https://github.com/noyzilla/oops/releases/latest/download/oopsbox.tar.gz`), extracting it cleanly into the target directory. If the release asset is unreachable, falls back to `https://github.com/noyzilla/oops/archive/refs/heads/main.tar.gz` (extracting `oopsbox/`).
- **Cryptographic Credential Generation**:
  - Automatically generates a 48-character cryptographic string for `OOPS_SECRET`.
  - Automatically generates a single shared 32-character secure password for `MYSQL_ROOT_PASSWORD`, `POSTGRES_PASSWORD`, and `REDIS_PASSWORD`.
  - Generates `.env` from template if not present.
- **Default Config Initialization**: Copies `oopsbox.yml.example` to `oopsbox.yml` if `oopsbox.yml` does not exist.
- **Automatic Active Box Registration**: Automatically writes the new workspace canonical path to `~/.oops/active_box`.

### Active Box Management (`oops box active [path]`)
- **Display Active Box**: Running `oops box active` without arguments prints the current active workspace path and its validation status.
- **Set Active Box**: Running `oops box active <path>` validates that the target path contains valid compose stacks (`stacks/`) and writes the path to `~/.oops/active_box`.

### Engine Runtime & Boot Lifecycle (`oops box start`)
- **Engine Auto-Detection & Boot (`engine.type`)**:
  - Checks `engine.type` in `oopsbox.yml` or `OOPS_ENGINE_TYPE` env var (default: `auto`).
  - `auto`: Prioritizes `orbstack` -> `colima` -> `docker`.
  - `orbstack`: Ensures OrbStack is running (`orb start` on macOS).
  - `colima`: Ensures Colima is running with configured CPU/Memory/Disk specs.
  - `docker`: Verifies Docker daemon connectivity (`docker info`).
- **Static DNS Mapping (`data/oops/dns.records`)**: Generates and updates `data/oops/dns.records` mapping `host.<tld>` to host gateway IP and `vm.<tld>` to container engine IP.
- **macOS Local DNS Resolver**: Configures `/etc/resolver/<tld>` pointing to local DNS listener (`127.0.0.1:53` or VM IP).
- **Default Profile Boot**: Automatically delegates to `oops up` to boot default services (`caddy-proxy`, `oops`, etc.).

### Multi-Box Switching (`oops box switch <path>`)
When switching between isolated organization workspaces:
- **Graceful Teardown**: Runs `oops down` on the currently active box to release shared container names (`caddy-proxy`, `mysql`, `postgres`, `redis`, `oops`) and host ports.
- **Zero Data Loss**: Persistent service data remains intact in `./data/`.
- **State Handover**: Sets `~/.oops/active_box` to the target path and invokes `oops box start` on the target workspace.

### CA Certificate Trust (`oops box cert`)
- Exports Caddy root CA (`root.crt`) from the Caddy container volume (`caddy_data` or `stacks/edge/data/caddy/pki/authorities/local/root.crt`).
- Installs root CA into macOS System Keychain (`security add-trusted-cert -d -r trustRoot`) or Linux system trust store (`/usr/local/share/ca-certificates/` and `update-ca-certificates`).

## Interface & Subcommands Specification

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| `oops box init` | `[path]` | Downloads latest release blueprint, generates secure credentials in `.env`, initializes `oopsbox.yml`, and sets active box |
| `oops box active` | `[path]` | Displays or sets the currently active Oopsbox workspace in `~/.oops/active_box` |
| `oops box list` | *(none)* | Lists known and active Oopsbox workspaces |
| `oops box start` | *(none)* | Boots container engine (OrbStack/Colima/Docker), configures resolver, syncs DNS records, and starts default stack \(`oops up`\) |
| `oops box stop` | *(none)* | Stops all running stacks via `oops down` on the active box |
| `oops box switch` | `<path>` | Gracefully tears down current box, updates active box, and starts target box |
| `oops box cert` | *(none)* | Installs local Caddy CA root certificate into host OS trust store / Keychain |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops box <cmd>`), CI/CD scripts.
- **Downstream Dependencies**: GitHub Releases API / curl, Docker Engine, OrbStack, Colima, macOS `/etc/resolver/`, macOS `security`, Linux `update-ca-certificates`.
- **Bounded Blast Radius**:
  - `cmd/box.go`: Cobra CLI subcommand definitions.
  - `internal/box/`: Workspace lifecycle, blueprint extraction, credential generator, engine controller.
  - `cmd/root.go`: Active box resolution & validation.
  - `install.sh`: Standalone CLI installer and completion setup.

## Verification & Acceptance Criteria

- **Unit Tests**:
  - `go test -v -race ./internal/box/... ./cmd/...` exits with Code 0.
  - Random credential generator produces 48-char alphanumeric secrets and 32-char passwords.
  - Active box resolution and validation functions handle non-existent, invalid, and valid directories.
- **CLI Behavior**:
  - `oops box --help` displays all subcommands cleanly.
  - `oops box active` displays active box and validation status.
  - Tab completion correctly suggests `oops box` and all subcommands (`create`, `active`, `list`, `start`, `stop`, `switch`, `cert`).
