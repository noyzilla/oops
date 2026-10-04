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
- **Upstream Forwarder**: Stateless relay to external DNS servers (configured in `oops.yml` under `dns.upstreams`) for unresolved public domains.

---

## Business Rules & Resolution Invariants

### Hostname Resolution Rules

DNS `A` record queries are matched against active container hostnames in the following priority:

- **Wildcard Hostname Match (`hostname: .<domain>`)**:
  - Condition: Container `Config.Hostname` starts with `.`.
  - Matching: If query equals `<domain>` (apex) OR query ends with `.<domain>` (any subdomain).
  - Action: Return `A` record with IPv4 address of the matching container (TTL: 5s).
  - Example: `hostname: .web.oops` matches `web.oops`, `app.web.oops`, `api.web.oops`.

- **Exact Hostname Match (`hostname: <domain>`)**:
  - Condition: Container `Config.Hostname` does not start with `.`.
  - Matching: If query exactly equals `<domain>` (case-insensitive).
  - Action: Return `A` record with IPv4 address of the matching container (TTL: 5s).
  - Example: `hostname: mail.test` matches `mail.test`.

- **Custom Static & Infra DNS Records (`data/oops/dns.records`, `oops.yml`, `OOPS_DNS_RECORDS`)**:
  - Local Node Data: Static mappings defined in `data/oops/dns.records` (Format: `<domain> <ip>`, 1 row = 1 domain, supports `.wildcard`).
  - Shared Infra Records: Defined under `dns.records` in `oops.yml` (e.g. `- api.internal 10.0.0.5`).
  - Environment Overrides: `OOPS_DNS_RECORDS` env var.
  - Matching: Exact hostnames (e.g. `host.oops`, `vm.oops`, `hostmac`) or wildcard prefixes (e.g. `.local.dev`, `.web.oops`).
  - Action: Return static mapped IPv4 address.
  - Example: `host.oops 192.168.64.1` maps workstation gateway, `vm.oops 192.168.64.2` maps Colima VM.
  - **Hot-Reload & Safety**: The daemon watches file modtimes (`WatchDNSConfigFile`) and atomic-swaps static records without downtime or daemon restart. Malformed lines are safely skipped with warning logs without corrupting the active routing table.

- **Fallback & No Hostname**:
  - If a container does not specify a domain-formatted hostname, Oops DNS does not invent arbitrary names.
  - If a query does not match any registered container hostname or static host mapping, it proceeds to Upstream Forwarding.

### Upstream DNS Relay Protocol
- If query does not match any local container hostname or custom static record:
  - Relay the query to upstream DNS servers defined in `oops.yml` (`dns.upstreams: [...]`) or `OOPS_DNS_UPSTREAM` (default: `1.1.1.1:53,8.8.8.8:53`).
  - Return the upstream response directly to the client.
  - If upstream resolution fails or times out, return `NXDOMAIN` (Rcode 3) or `SERVFAIL`.

---

## Docker Events & File Synchronization State Machine

The DNS resolver table is maintained dynamically via the Docker Engine API and static config file watcher:

```
[Daemon Startup] ──> Full Container List Scan ──> Register Running Container Hostnames
       │
       ├──> Load data/oops/dns.records + oops.yml ──> Atomic Register Static & Wildcard Records
       │
       ▼
[Event Watchers Loop]
       ├── Docker Event ("start" / "die") ──> Register/Remove Container Hostname
       └── File Watcher (ModTime check)   ──> Atomic Swap Static Records (data/oops/dns.records)
```

- **Thread-Safety**: Hostname lookup and mutation use read/write locking (`sync.RWMutex`).
- **Multi-Network IP Resolution**: Uses the primary network IP (preferring `net-edge`, `net-apps`, or bridge).

---

## DNS Inspection & Management CLI (`oops dns`)

- **`oops dns` / `oops dns list`**: Inspects and tabulates active DNS records, separating Custom & Infra records from Service Container records.
- **`oops dns get <domain>`**: Queries and outputs the resolved IP for a domain directly to stdout (compatible with shell scripts and automation).
- **`oops dns set <domain> <ip>`**: Adds or updates a custom record in `data/oops/dns.records` (supports `.wildcard`, e.g. `.my-app.test 127.0.0.1`). Validates and prevents overwriting active container services.
- **`oops dns del <domain>`**: Removes a custom record from `data/oops/dns.records`. Rejects deletion if domain is an active container service.
- **`oops dns reload`**: Validates file syntax and triggers atomic reload across running daemon.

---

| Environment Variable / Config Key | Internal Config Path | Type | Default | Description |
| :--- | :--- | :---: | :---: | :--- |
| `dns.upstreams` (`oops.yml`) / `OOPS_DNS_UPSTREAM` | `CONFIG.DNS.UPSTREAM` | `[]string` / `string` | `1.1.1.1:53,8.8.8.8:53` | Upstream DNS servers for public relay |
| `OOPS_DNS_TLD` | `CONFIG.DNS.TLD` | `string` | `oops` | Node top-level domain for local resolver and static records |

---

## Verification & Acceptance Criteria

- **Unit Tests**:
  - `TestResolver_ExactMatch`: Verifies `mail.test` resolves to registered IP.
  - `TestResolver_WildcardMatch`: Verifies `.web.oops` resolves `web.oops`, `app.web.oops`, `deep.sub.web.oops`.
  - `TestResolver_CaseInsensitive`: Verifies `APP.WEB.OOPS` matches `.web.oops`.
  - `TestResolver_Unregister`: Verifies container teardown removes hostnames cleanly.
