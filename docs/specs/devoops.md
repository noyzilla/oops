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

`devoops` is the developer workstation suite located in `templates/oopsbox/bin/` (`bin/devoops`, `bin/devoops-mac`, `bin/devoops-linux`). It orchestrates local development environments on macOS and Linux desktop machines, abstracting VM provisioning (Colima / Lima), host firewall packet filtering (`pfctl` / `iptables`), loopback routing, local DNS resolver integration (`/etc/resolver/`), and root CA certificate trust for local TLS wildcard domains (`https://*.web.oops` or `https://*.test`).

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

- **Colima Integration & VM Optimization**: If Docker Desktop is not active, utilizes Colima VM with designated CPU, Memory, and VZ optimizations, configuring internal kernel sysctl, disabling conflicting port 53 listeners, and setting DNS resolvers.
- **Static Host Mapping (`config/oops/hosts`)**: Auto-generates and maintains `config/oops/hosts` mapping `host.oops` to the macOS workstation host IP (gateway) and `vm.oops` to the Colima Linux VM IP.
- **Direct Bridge / Route**: Configures host-to-VM routing (e.g. `10.200.0.0/16` or Colima interface IP) so containers can be reached directly via IP or reverse proxy.
- **Host Firewall Anchors (`pfctl`)**: Binds local ports (80/443/53) or forwards traffic into the local Docker subnet via dedicated packet filter rules (`/etc/pf.anchors/oopsbox`).
- **macOS Local DNS Resolver**: Creates `/etc/resolver/test` or `/etc/resolver/oops` pointing to `127.0.0.1:53` or container DNS IP to ensure local subdomains resolve seamlessly without `/etc/hosts` pollution.
- **Keychain Local SSL CA**: Extracts `root.crt` from the Caddy container volume and installs it into macOS System Keychain (`security add-trusted-cert -d -r trustRoot`) to guarantee green HTTPS locks in Google Chrome, Safari, and Curl.

### Linux Native Dev Rules
- Detects local Docker Engine daemon (`systemctl status docker`).
- Installs root CA into system trust store (`/usr/local/share/ca-certificates/` or `/etc/ca-certificates/trust-source/anchors/` and runs `update-ca-certificates`).
- Adds local systemd-resolved or NetworkManager DNS forwarder.

## Interface & Subcommands Specification

| Subcommand | Aliases | Description |
| :--- | :--- | :--- |
| `devoops init` | `install`, `setup` | Automated workstation setup: initializes storage, `.env`, VM, DNS resolver, and symlinks `devoops` to global PATH (`/usr/local/bin/devoops`) |
| `devoops start` | `up` | Starts VM daemon, network routing, and launches Default Service Group (`@core`) |
| `devoops stop` | `down` | Stops services and suspends background dev VM |
| `devoops restart` | *(none)* | Restarts VM daemon and service containers |
| `devoops status` | `info` | Displays status of VM, Docker engine, network routes, DNS resolver, and running containers |
| `devoops install-cert` | `cert` | Exports Caddy local root CA from container volume and installs it into OS Trust Store |
| `devoops hosts` | `dns` | Prints active local DNS mappings and resolver configuration |
| `devoops logs` | *(none)* | Displays logs from VM daemon or container logs |
| `devoops <command>` | *(passthrough)* | Any unrecognized command is passed directly to the `oops` CLI binary |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal, onboarding scripts, local dev workflows.
- **Downstream Dependencies**: Docker Engine, Colima (`colima`), macOS `pfctl`, macOS `security` (Keychain), `sudo`, Caddy PKI volume.
- **Bounded Blast Radius**:
  - `templates/oopsbox/bin/devoops`: OS router.
  - `templates/oopsbox/bin/devoops-mac`: macOS driver.
  - `templates/oopsbox/bin/devoops-linux`: Linux driver.
  - `templates/oopsbox/README.md`: Workstation developer documentation.

## Verification & Acceptance Criteria

- **Syntax Validation**: `sh -n templates/oopsbox/bin/devoops` and `bash -n templates/oopsbox/bin/devoops-mac` and `bash -n templates/oopsbox/bin/devoops-linux` exit with Code 0.
- **Execution Scenarios**:
  - Running `bin/devoops --help` displays all subcommands cleanly.
  - Running `bin/devoops status` inspects OS and passes through status.
  - Passthrough commands (`bin/devoops up @core`, `bin/devoops status`) execute equivalent `oops` operations.
