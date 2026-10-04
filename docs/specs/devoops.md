---
title: DevOops Workstation Tooling Suite
status: active
tags: [devoops, local-dev, macos, linux, colima, resolver, ssl-ca]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/cli.md", "docs/specs/dns.md"]
---

# Specification: DevOops Workstation Tooling Suite

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/cli.md](cli.md), [docs/specs/dns.md](dns.md)

## Overview & Scope

`devoops` is the developer workstation suite located in `oopsbox/bin/` (`bin/devoops`, `bin/devoops-mac`, `bin/devoops-linux`). It orchestrates local development environments on macOS and Linux desktop machines, abstracting VM provisioning (Colima / Lima), host firewall packet filtering (`pfctl` / `iptables`), loopback routing, local DNS resolver integration (`/etc/resolver/`), and root CA certificate trust for local TLS wildcard domains (`https://*.web.oops` or `https://*.test`).

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **DevOops Dispatcher (`bin/devoops`)**: OS-detecting entrypoint script that delegates to OS-specific drivers (`devoops-mac` or `devoops-linux`).
- **VM Driver (`devoops-mac`)**: macOS-specific management script managing Colima/Lima VM, port forwarders, firewall anchors, and Keychain root certificate insertion.
- **Linux Driver (`devoops-linux`)**: Linux-native management script managing systemd Docker services, iptables routing, and desktop NSS certificate databases.
- **Local TLD Resolver**: macOS `/etc/resolver/<domain>` entry forwarding local domain resolution to the Oops embedded DNS watcher.
- **Local Root CA**: Caddy-generated internal root certificate installed into the host OS trust store.

## Business Rules & Logic Invariants

### OS Dispatch Protocol
- `bin/devoops` detects host operating system via `uname -s`.
- On `Darwin` (macOS), invokes `bin/devoops-mac`.
- On `Linux`, invokes `bin/devoops-linux`.
- On unsupported operating systems (Windows/WSL without Linux shell), outputs an actionable error message and exits with Code 1.

### Multi-Engine Runtime & VM Optimization
- **Multi-Engine Runtime Selection (`engine.type`)**: Configured via `engine.type` in `devoops.yml` (copied from `devoops.yml.example` at root) or `OOPS_ENGINE_TYPE` environment variable:
  - `auto` (Default): Autodetects available engines in priority order: `orbstack` -> `colima` -> `docker`.
  - `orbstack`: Launches OrbStack VM (`orb start`), providing native direct container IP routing without requiring `iptables` or manual `route add` configurations.
  - `colima`: Launches Colima VM with configured CPU, Memory, Disk, and VZ hypervisor, applying internal kernel sysctl, firewall watchers, and subnet route bindings.
  - `docker`: Directly connects to active Docker daemon or Docker Desktop.
- **Configurable DNS TLD (`dns.tld`)**: Configured in `devoops.yml` (`dns.tld`) or `OOPS_DNS_TLD` (default: `oops`).
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
| `devoops install` | *(none)* | Automated workstation setup: initializes storage, `.env`, VM/routes, and registers `devoops` to global PATH |
| `devoops start` | `[profile]` | Starts VM/daemon, network routing, and launches default or specified profile (e.g. `@default`, `@lab`) |
| `devoops install-cert` | *(none)* | Exports Caddy local root CA from container volume and installs it into OS Trust Store |
| `devoops stop` | *(none)* | Stops services and shuts down running stacks |
| `devoops status` | *(none)* | Displays status of running containers |
| `devoops dns` | *(none)* | Prints active local DNS mappings and resolver configuration |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal, onboarding scripts, local dev workflows.
- **Downstream Dependencies**: Docker Engine, Colima (`colima`), macOS `pfctl`, macOS `security` (Keychain), `sudo`, Caddy PKI volume.
- **Bounded Blast Radius**:
  - `oopsbox/bin/devoops`: OS router.
  - `oopsbox/bin/devoops-mac`: macOS driver.
  - `oopsbox/bin/devoops-linux`: Linux driver.
  - `oopsbox/README.md`: Workstation developer documentation.

## Verification & Acceptance Criteria

- **Syntax Validation**: `sh -n oopsbox/bin/devoops` and `bash -n oopsbox/bin/devoops-mac` and `bash -n oopsbox/bin/devoops-linux` exit with Code 0.
- **Execution Scenarios**:
  - Running `bin/devoops --help` displays all subcommands cleanly.
  - Running `bin/devoops status` inspects OS and passes through status.
  - Passthrough commands (`bin/devoops up @default`, `bin/devoops status`) execute equivalent `oops` operations.
