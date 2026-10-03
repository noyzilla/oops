---
title: Native DNS Daemon & Service Discovery Living Specification
status: active
tags: [dns, discovery, hostname, wildcard, server]
synapses: ["docs/specs/webhook.md", "docs/specs/cli.md"]
---

# Native DNS Daemon & Service Discovery [ระบบบริการดีเอ็นเอสและค้นหาเซอร์วิส]

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Native DNS Engine**: High-performance, lightweight UDP/TCP DNS server running embedded inside `oops server` (listening on `:53`).
- **Wildcard Hostname**: Container hostname prefixed with a dot `.` (e.g. `hostname: .web.oops`), matching both apex and all subdomains (`web.oops` and `*.web.oops`).
- **Exact Hostname**: Container hostname without leading dot (e.g. `hostname: mail.test`), matching only that exact domain.
- **Upstream Forwarder**: Stateless relay to external DNS servers (`OOPS_DNS_UPSTREAM`) for unresolved public domains.

---

## Business Rules & Resolution Invariants

### 1. Hostname Resolution Rules

DNS `A` record queries are matched against active container hostnames in the following priority:

1. **Wildcard Hostname Match (`hostname: .<domain>`)**:
   - Condition: Container `Config.Hostname` starts with `.`.
   - Matching: If query equals `<domain>` (apex) OR query ends with `.<domain>` (any subdomain).
   - Action: Return `A` record with IPv4 address of the matching container (TTL: 5s).
   - Example: `hostname: .web.oops` matches `web.oops`, `app.web.oops`, `api.web.oops`.

2. **Exact Hostname Match (`hostname: <domain>`)**:
   - Condition: Container `Config.Hostname` does not start with `.`.
   - Matching: If query exactly equals `<domain>` (case-insensitive).
   - Action: Return `A` record with IPv4 address of the matching container (TTL: 5s).
   - Example: `hostname: mail.test` matches `mail.test`.

3. **Custom Static Hosts Resolution (`stacks/utils/config/oops/hosts` & `OOPS_DNS_RECORDS`)**:
   - Condition: Static mappings defined in `stacks/utils/config/oops/hosts` (standard `/etc/hosts` format) or `OOPS_DNS_RECORDS` env var.
   - Matching: Exact hostnames (e.g. `hostmac`, `host.docker.internal`, `colima`) or wildcard suffixes (e.g. `.local.dev`).
   - Action: Return static mapped IPv4 address.
   - Example: `192.168.5.2 hostmac host.docker.internal hostdocker` maps Colima/host machine gateway.

4. **Fallback & No Hostname**:
   - If a container does not specify a domain-formatted hostname, Oops DNS does not invent arbitrary names.
   - If a query does not match any registered container hostname or static host mapping, it proceeds to Upstream Forwarding.

### 2. Upstream DNS Relay Protocol
- If query does not match any local container hostname or custom static record:
  - Relay the query to upstream DNS servers defined in `OOPS_DNS_UPSTREAM` (default: `1.1.1.1:53,8.8.8.8:53`).
  - Return the upstream response directly to the client.
  - If upstream resolution fails or times out, return `NXDOMAIN` (Rcode 3) or `SERVFAIL`.

---

## Docker Events Synchronization State Machine

The DNS resolver table is maintained dynamically via the Docker Engine API:

```
[Daemon Startup] ──> Full Container List Scan ──> Register Running Container Hostnames
       │
       ▼
[Docker Event Loop]
       ├── Event "start" / "unpause"   ──> Inspect Container ──> Register/Update Hostname & IP
       └── Event "die" / "stop" / "pause" / "destroy" ──> Remove Hostname from Table
```

- **Thread-Safety**: Hostname lookup and mutation use read/write locking (`sync.RWMutex`).
- **Multi-Network IP Resolution**: Uses the primary network IP (preferring `net-edge`, `net-apps`, or bridge).

---

## Configuration Parity

| Environment Variable | Internal Config Path | Type | Default | Description |
| :--- | :--- | :---: | :---: | :--- |
| `OOPS_DNS_UPSTREAM` | `CONFIG.DNS.UPSTREAM` | `string` | `1.1.1.1:53,8.8.8.8:53` | Comma-separated upstream DNS servers |

---

## Verification & Acceptance Criteria

- **Unit Tests**:
  - `TestResolver_ExactMatch`: Verifies `mail.test` resolves to registered IP.
  - `TestResolver_WildcardMatch`: Verifies `.web.oops` resolves `web.oops`, `app.web.oops`, `deep.sub.web.oops`.
  - `TestResolver_CaseInsensitive`: Verifies `APP.WEB.OOPS` matches `.web.oops`.
  - `TestResolver_Unregister`: Verifies container teardown removes hostnames cleanly.
