---
title: IAP SSH Database Tunnel & Dynamic Service Discovery Living Specification
status: active
tags: [tunnel, iap, gcp, ssh, zero-port, dns, remote, database]
synapses: ["docs/specs/remote.md", "docs/specs/dns.md", "docs/specs/cli.md", "CONTEXT.md"]
---

# Specification: IAP SSH Database Tunnel & Dynamic Service Discovery [ระบบเชื่อมต่อ Database ผ่าน IAP Tunnel และ DNS ภายใน]

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/remote.md](remote.md), [docs/specs/dns.md](dns.md), [docs/specs/cli.md](cli.md)

## Overview & Scope

This specification defines the `oops tunnel` subsystem. It provides secure, seamless remote database access (such as MySQL, Redis, PostgreSQL) to remote server environments (`@<remote>`, e.g. `@b9tpe`) without exposing server IP addresses and without exposing any host ports—either on the remote server or on the local development workstation.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Client Tunnel Container**: Isolated local Docker container (`google/cloud-sdk:alpine`) managed by Oops on the local Docker network (`oops_edge`/`bridge`), executing the IAP SSH client and listening on service ports.
- **Dynamic Service-to-IP Resolution**: Technique of querying the container IP of a remote Docker Compose service (e.g., `mysql`, `redis`) directly on the remote server's Docker bridge network, bypassing remote host port mappings.
- **Zero Remote Host Port Exposure**: Invariant stating remote servers map zero database ports (`3306`, `6379`) to host interfaces (`127.0.0.1` or `0.0.0.0`). Communication is routed solely through internal container bridge IPs.
- **Zero Local Host Port Exposure**: Invariant stating local development workstations map zero tunnel ports to `localhost`. Port bindings exist exclusively within the Client Tunnel Container, accessed via Oops DNS (`tunnel-<remote>.oops:<port>`).
- **IAP SSH Gateway**: Google Cloud Identity-Aware Proxy (IAP) TCP forwarding over TLS port 443, eliminating direct public IP exposure of the remote host.

---

## Business Rules & Logic Invariants

### 1. Network Security & IP Masking Invariant
- Direct connection to the remote server's public or private IP from the local workstation is strictly prohibited.
- All tunnel traffic must route through Google Cloud IAP (`gcloud compute start-iap-tunnel` or `gcloud compute ssh --tunnel-through-iap`) over outbound HTTPS (TCP port 443).

### 2. Zero Host Port Exposure (Remote Server)
- Remote database containers run without published ports in `compose.yaml` (no `ports:` section).
- When a tunnel is requested for service `mysql:3306`, the CLI resolves the remote container's internal IPv4 address (e.g. `172.18.0.5`) on the remote Docker network.
- The SSH port forward directive binds directly to the container IP: `-L 0.0.0.0:3306:172.18.0.5:3306`.

### 3. Zero Host Port Exposure & DNS Isolation (Local Dev Workstation)
- The tunnel client runs inside a dedicated local Docker container named `oops-tunnel-<remote>` (e.g. `oops-tunnel-b9tpe`).
- The container attaches to the Oops Docker network (e.g. `oops_edge`, `oops_db`, or `bridge`).
- The container is assigned:
  - Container Hostname: `tunnel-<remote>.oops`
  - DNS Labels:
    - `oops.dns.hostname: .<remote>.tunnel.oops,tunnel-<remote>.oops`
    - `oops.dns.mappings: <service1>.<remote>.tunnel.oops=<ip1>,<service2>.<remote>.tunnel.oops=<ip2>`
- Developers and database GUIs connect via dedicated service domains:
  - `<service>.<remote>.tunnel.oops:<port>` (e.g. `mysql.b9tpe.tunnel.oops:3306`, `redis.b9tpe.tunnel.oops:6379`)
  - Backward-compatible shorthand: `tunnel-<remote>.oops:<port>`
- **Port Collision Invariant (Secondary IP Aliasing)**:
  - When multiple services share the same local port (e.g. `mysql1:3306` and `mysql2:3306`), the single tunnel container uses `--cap-add=NET_ADMIN` to bind secondary IP aliases (`ip addr add <ip>/16 dev eth0`).
  - Oops DNS resolves `mysql1.<remote>.tunnel.oops` to IP 1 and `mysql2.<remote>.tunnel.oops` to IP 2.
  - SSH runs a single session forwarding each local IP:port directly to its respective remote container.
- This guarantees zero port clashes on macOS `localhost`, allowing concurrent tunnels to multiple remote environments (`@b9tpe`, `@prod`, `@staging`) and multiple services with identical ports simultaneously.

### 4. Configuration Resolution (`@<remote>/remote.yaml`)
- The tunnel configuration is loaded automatically from `/@<remote>/remote.yaml` in the active workspace.
- Attributes:
  - `host`: Remote SSH host alias or address (e.g. `b9tpe` configured in `~/.ssh/config`, or `user@host`).
  - `path`: Remote oopsbox path (default: `~/oopsbox`).
  - `tunnel.services`: List of default services to forward when ports are omitted (e.g. `[mysql, redis]`).
- Example `remote.yaml`:
  ```yaml
  host: b9tpe
  path: ~/oopsbox
  tunnel:
    services:
      - mysql
      - redis
  ```

### 5. Port Specification Grammar & Dynamic Port Discovery
- Arguments specify service targets: `<service>`, `<service>:<port>`, or `<service>:<local_port>:<remote_port>`.
- **Dynamic Port Auto-Discovery**:
  - When port is omitted (e.g. `mysql` or listed in `tunnel.services`), the CLI inspects the remote container's `Config.ExposedPorts` (e.g. `3306/tcp`) and automatically maps it.
  - Developers do not need to memorize or pass port numbers for standard services.
- Priority:
  - CLI arguments take precedence.
  - If CLI arguments are empty, `tunnel.services` from `remote.yaml` is used.
  - If `remote.yaml` has no `tunnel.services`, defaults to `[mysql, redis]`.

### 6. Cloud-Agnostic SSH Transport
- Tunneling uses standard OpenSSH client commands (`ssh <host>`), completely decoupled from cloud-specific CLI flags.
- For GCP instances, `~/.ssh/config` defines `ProxyCommand gcloud compute start-iap-tunnel ...` which OpenSSH executes seamlessly.
- For non-GCP hosts (AWS, Hetzner, Bare-Metal), standard SSH connects directly without configuration changes.
- **macOS SSH Config Sanitization**: When mounting `~/.ssh` from macOS host machines into the Linux tunnel container, Apple-specific options (such as `UseKeychain yes`) are automatically stripped from the container's local `/root/.ssh/config` copy and overridden via `-o IgnoreUnknown=UseKeychain` to prevent OpenSSH option rejection crashes.

---

## Flow & State Machine

```
[Local Dev Terminal]
       │
       ▼ oops tunnel up @b9tpe [mysql:3306 redis:6379]
1. Load /@b9tpe/remote.yaml (host, zone, project)
2. Execute remote Docker inspect via IAP SSH:
       │ query remote container IPs (e.g. mysql -> 172.18.0.5)
       ▼
3. Ensure local network exists (oops_edge / bridge)
4. Spawn Client Tunnel Container:
       ├── Image: google/cloud-sdk:alpine
       ├── Container Name: oops-tunnel-b9tpe
       ├── Hostname & Label: tunnel-b9tpe.oops, b9tpe.oops
       ├── Volumes: ~/.config/gcloud:/root/.config/gcloud:ro, ~/.ssh:/root/.ssh-host:ro
       └── Command: gcloud compute ssh <host> --zone=<zone> --tunnel-through-iap -- \
                    -N -g -L 0.0.0.0:3306:172.18.0.5:3306 -L 0.0.0.0:6379:172.18.0.6:6379 ...
       ▼
5. Local Oops DNS daemon registers tunnel-b9tpe.oops -> Tunnel Container IP
6. Output connection endpoints table to user
```

### Tunnel Lifecycle States
- **Down**: Tunnel container does not exist.
- **Starting**: Container created, performing SSH handshake and port forwarding.
- **Up / Healthy**: Container is running, SSH port forwarding active, Oops DNS resolves domain.
- **Failed**: Resolution failed or port already bound inside container.

---

## Interface & Data Contracts

### CLI Commands
- `oops tunnel @<remote> [service:port...]`: Shorthand to start tunnel if down, or show status if up.
- `oops tunnel up @<remote> [service:port...]`: Explicitly start/replace the tunnel container.
- `oops tunnel down @<remote>`: Stop and remove the tunnel container.
- `oops tunnel list` (aliases: `ps`, `ls`): List all active tunnels, remote targets, forwarded ports, and DNS endpoints.
- `oops tunnel restart @<remote> [service:port...]`: Re-resolve remote container IPs and restart tunnel container.

### Status Output Schema
```text
REMOTE   CONTAINER            STATUS    ENDPOINT                 FORWARDED SERVICES
@b9tpe   oops-tunnel-b9tpe    Up (2m)   tunnel-b9tpe.oops:3306   mysql (172.18.0.5:3306)
                                        tunnel-b9tpe.oops:6379   redis (172.18.0.6:6379)
```

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers (Inbound)**: `cmd/tunnel.go`, `cmd/root.go`.
- **Downstream Dependencies (Outbound)**:
  - `internal/tunnel`: Core orchestrator for tunnel container and IAP resolver.
  - `internal/remote`: `remote.LoadConfig`, `remote.Config` (with `Zone`, `Project`).
  - `internal/dns`: `ExtractContainerHostnames`, `Resolver`.
  - Docker Engine API (`github.com/docker/docker/client`).
- **Bounded Blast Radius**:
  - `cmd/tunnel.go`, `cmd/tunnel_test.go`
  - `internal/tunnel/*.go`
  - `internal/remote/config.go`, `internal/remote/config_test.go`
  - `internal/dns/watcher.go`, `internal/dns/inspector.go`

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v ./internal/tunnel/... ./internal/remote/... ./internal/dns/... ./cmd/...`
- **Acceptance Scenarios**:
  - `TestRemoteConfig_ZoneProject`: Loads and parses `zone` and `project` from `remote.yaml`.
  - `TestParsePortSpecs`: Correctly parses `mysql:3306`, `redis:6379`, `pg:5432:5432` and applies defaults.
  - `TestDNSMultiHostnameSupport`: Extracts and registers multiple comma-delimited hostnames from `oops.dns.hostname`.
  - `TestTunnelContainerConfig`: Builds valid Docker container config, labels, mounts, and SSH forwarding flags.

