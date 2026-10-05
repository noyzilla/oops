---
title: Oopsbox Workstation Tooling Suite
status: active
tags: [oopsbox, local-dev, macos, linux, colima, resolver, ssl-ca]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/cli.md", "docs/specs/dns.md"]
---

# Specification: Oopsbox Workstation Tooling Suite

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/cli.md](cli.md), [docs/specs/dns.md](dns.md)

## Overview & Scope

`oopsbox` is the developer workstation suite located in `oopsbox/bin/` (`bin/oopsbox`, `bin/oopsbox-mac`, `bin/oopsbox-linux`). It orchestrates local development environments on macOS and Linux desktop machines, abstracting VM provisioning (Colima / Lima), host firewall packet filtering (`pfctl` / `iptables`), loopback routing, local DNS resolver integration (`/etc/resolver/`), and root CA certificate trust for local TLS wildcard domains (`https://*.web.oops` or `https://*.test`).

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Oopsbox Dispatcher (`bin/oopsbox`)**: OS-detecting entrypoint script that delegates to OS-specific drivers (`oopsbox-mac` or `oopsbox-linux`).
- **VM Driver (`oopsbox-mac`)**: macOS-specific management script managing Colima/Lima VM, port forwarders, firewall anchors, and Keychain root certificate insertion.
- **Linux Driver (`oopsbox-linux`)**: Linux-native management script managing systemd Docker services, iptables routing, and desktop NSS certificate databases.
- **Local TLD Resolver**: macOS `/etc/resolver/<domain>` entry forwarding local domain resolution to the Oops embedded DNS watcher.
- **Local Root CA**: Caddy-generated internal root certificate installed into the host OS trust store.

## Business Rules & Logic Invariants

### OS Dispatch Protocol
- `bin/oopsbox` detects host operating system via `uname -s`.
- On `Darwin` (macOS), invokes `bin/oopsbox-mac`.
- On `Linux`, invokes `bin/oopsbox-linux`.
- On unsupported operating systems (Windows/WSL without Linux shell), outputs an actionable error message and exits with Code 1.

### Multi-Engine Runtime & VM Optimization
- **Multi-Engine Runtime Selection (`engine.type`)**: Configured via `engine.type` in `oopsbox.yml` (copied from `oopsbox.yml.example` at root) or `OOPS_ENGINE_TYPE` environment variable:
  - `auto` (Default): Autodetects available engines in priority order: `orbstack` -> `colima` -> `docker`.
  - `orbstack`: Launches OrbStack VM (`orb start`), providing native direct container IP routing without requiring `iptables` or manual `route add` configurations.
  - `colima`: Launches Colima VM with configured CPU, Memory, Disk, and VZ hypervisor, applying internal kernel sysctl, firewall watchers, and subnet route bindings.
  - `docker`: Directly connects to active Docker daemon or Docker Desktop.
- **Configurable DNS TLD (`dns.tld`)**: Configured in `oopsbox.yml` (`dns.tld`) or `OOPS_DNS_TLD` (default: `oops`).
- **Static DNS Mapping (`data/oops/dns.records`)**: Auto-generates and maintains `data/oops/dns.records` mapping `host.<tld>` to the workstation host IP (gateway) and `vm.<tld>` to the VM/engine IP.
- **Direct Bridge / Route**: Configures host-to-VM routing (e.g. `10.200.0.0/16` or Colima interface IP) when running under Colima so containers can be reached directly via IP or reverse proxy.
- **Host Firewall Anchors (`pfctl`)**: Binds local ports (80/443/53) or forwards traffic into the local Docker subnet via dedicated packet filter rules (`/etc/pf.anchors/oopsbox`).
- **macOS Local DNS Resolver**: Creates `/etc/resolver/<tld>` pointing to the container DNS IP to ensure local subdomains resolve seamlessly without `/etc/hosts` pollution.
- **Keychain Local SSL CA**: Extracts `root.crt` from the Caddy container volume and installs it into macOS System Keychain (`security add-trusted-cert -d -r trustRoot`) to guarantee green HTTPS locks in Google Chrome, Safari, and Curl.

### Linux Native Dev Rules
- Detects local Docker Engine daemon (`systemctl status docker`).
- Installs root CA into system trust store (`/usr/local/share/ca-certificates/` or `/etc/ca-certificates/trust-source/anchors/` and runs `update-ca-certificates`).
- Adds local systemd-resolved or NetworkManager DNS forwarder.

## Interface & Subcommands Specification

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| `oopsbox install` | *(none)* | Automated workstation setup: initializes storage, `.env`, VM/routes, DNS resolver, and registers `oopsbox` to global PATH |
| `oopsbox start` | *(none)* | Boots VM/daemon, configures network/DNS, and starts default oops runtime (`oops up`) |
| `oopsbox install-cert` | *(none)* | Exports Caddy local root CA from container volume and installs it into OS Trust Store / macOS Keychain |
| `oopsbox stop` | *(none)* | Stops services and shuts down running stacks via `oops down` to release container names and ports |
| `oopsbox switch` | `<path>` | Multi-box handover: Tears down current active workspace (`oops down`), re-links global CLI, and boots target Oopsbox environment |

### Multi-Box Switching Invariant (`oopsbox switch <path>`)
When switching between isolated organization workspaces (e.g. `~/Workspaces/org-a/oopsbox` -> `~/Workspaces/org-b/oopsbox`):
- **Container Name Cleanup**: Fixed container names (`caddy-proxy`, `mysql`, `postgres`, `redis`, `oops`) cannot coexist across multiple workspaces. `oopsbox switch` executes a full graceful `down` on the current workspace before booting the target workspace.
- **Zero Data Loss**: Because service data is persisted in host directories (`./data/`), tearing down containers does not delete databases or persistent volumes.
- **Active Box State Recording (`~/.oops/active_box`)**: Writes absolute path of active workspace to `~/.oops/active_box` on `oopsbox start`, `oopsbox switch`, and `oopsbox install`. The `oops` CLI inspects this state to prevent accidental cross-box collisions.
- **Global Symlink Handover**: Atomically updates `/usr/local/bin/oopsbox` to target the active Oopsbox's `bin/oopsbox`.

> **Note**: For managing stacks, profiles, container health, logs, and DNS records, developers use `oops <command>` directly (e.g. `oops up @lab`, `oops switch`, `oops status`, `oops dns`).

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal, onboarding scripts, local dev workflows.
- **Downstream Dependencies**: Docker Engine, Colima (`colima`), macOS `pfctl`, macOS `security` (Keychain), `sudo`, Caddy PKI volume.
- **Bounded Blast Radius**:
  - `oopsbox/bin/oopsbox`: OS router.
  - `oopsbox/bin/oopsbox-mac`: macOS driver.
  - `oopsbox/bin/oopsbox-linux`: Linux driver.
  - `oopsbox/README.md`: Workstation developer documentation.

## Verification & Acceptance Criteria

- **Syntax Validation**: `sh -n oopsbox/bin/oopsbox` and `bash -n oopsbox/bin/oopsbox-mac` and `bash -n oopsbox/bin/oopsbox-linux` exit with Code 0.
- **Execution Scenarios**:
  - Running `bin/oopsbox` or `bin/oopsbox --help` displays all subcommands cleanly (including `switch`).
  - Running `bin/oopsbox start` boots VM engine, configures networking/resolver, and delegates to `oops up`.
  - Running `bin/oopsbox stop` delegates to `oops down`.
  - Running `bin/oopsbox switch <path>` tears down current containers and starts the target Oopsbox cleanly without port or name collisions.
