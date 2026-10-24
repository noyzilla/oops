---
id: "exp-2610-1"
title: "Mesh VPN & Hierarchical IP Addressing over SSH (remote.oops)"
status: active
tags: [exploration, vpn, mesh, ssh, ipam, dns, tunnel]
crystallization:
  target: undecided
synapses:
  - "docs/specs/tunnel.md"
  - "docs/specs/remote.md"
  - "docs/specs/dns.md"
---

# exp-2610-1: Mesh VPN & Hierarchical IP Addressing over SSH (remote.oops)

## Problem & Context

Currently, the `oops tunnel` subsystem operates via standard OpenSSH local port forwarding (`-L local:remote:port`). While this reliably exposes individual databases (such as MySQL and Redis) over Google Cloud IAP SSH without exposing public IPs, it introduces operational friction as remote infrastructures scale:

- **Per-Service Configuration Overhead**: Every remote service must be individually enumerated and port-forwarded (`mysql:3306`, `redis:6379`, `custom:8080`). Adding or deploying a new service requires updating configurations and restarting the tunnel.
- **Port Clashes & Port Management**: Multiple remotes with the same service port require either separate local IP aliases or complex port shifting.
- **Fragmented DNS Naming**: Endpoints are bound to `<service>.<remote>.tunnel.oops:<port>` rather than native direct service endpoints.
- **Subnet Collision Across Multiple Remotes**: If multiple remotes (e.g., `@b9tpe` and `@anthole`) are connected concurrently, both commonly use identical default Docker bridge subnets (e.g., `172.18.0.0/16` or `172.17.0.0/16`), making straightforward L3 IP routing collide on the client workstation.

---

## Motivation & Objectives

1. **Transparent Mesh Network**: Establish a user-space L3 VPN over SSH/IAP, allowing direct access to any container across all remote Docker networks without manual port-forwarding declarations.
2. **Native Domain Ergonomics (`<service>.<remote>`)**: Enable developers to access any container simply by direct domain and service port (e.g., `mysql.b9tpe:3306`, `httpbin.anthole:80`, `grafana.b9tpe:3000`).
3. **Collision-Free Multi-Host IPAM**: Establish a deterministic, multi-tenant IP addressing scheme that seamlessly handles multiple remote servers and multiple Docker networks per server without IP conflicts.
4. **Zero Cloud Firewall / Root Modifications**: Function entirely over standard SSH (and Google Cloud IAP SSH) without requiring `sshd_config` modifications (`PermitTunnel yes`), open public inbound ports, or root access on remote servers.

---

## Confirmed Hierarchical IPAM Schema (`10.<node_id>.<net_id>.<container_id>`)

To resolve IP collisions across concurrent remotes, internal Docker networks, cloud providers, and physical enterprise LANs, we establish a 4-octet hierarchical addressing schema:

$$\text{IP} = 10 \;.\; \langle\text{Node ID}\rangle \;.\; \langle\text{Docker Network ID: 0 - 254}\rangle \;.\; \langle\text{Container ID: 2 - 254}\rangle$$

| Octet | Segment | Range | Purpose |
| :--- | :--- | :---: | :--- |
| **Octet 1** | Root Prefix | `10` | Dedicated Class A private network block for Oops Mesh. |
| **Octet 2** | `Node ID` | `1 - 254` | Unique identifier for each host/machine. Configurable by user via `x-oops.network.subnet` in `oops.yaml`. **Default fallback: `240 - 254`**. |
| **Octet 3** | `Docker Net ID` | `0 - 254` | Distinct Docker network on that host (`1` = `oops_default`, `2` = `oops_db`, `3` = `apps_net`). |
| **Octet 4** | `Container Host ID` | `2 - 254` | Specific container host address within that Docker network. |

### Host Subnet & TLD Configuration in `oops.yaml`

Each Oopsbox host explicitly declares its authoritative CIDR block under `x-oops.network.subnet`, and locks its TLD to its remote identity:

```yaml
x-oops:
  network:
    subnet: 10.240.0.0/16       # Safe default for Cloud (e.g. b9tpe)
    # subnet: 10.50.0.0/16      # Or host-aligned subnet for office LAN (10.69.69.50)
  dns:
    tld: b9tpe                  # Remote Identity Invariant: tld == <remote_name> (e.g. b9tpe, anthole, oops)
```

#### Deterministic Infrastructure IP Allocation:
Within the declared `network.subnet` (e.g. `10.240.0.0/16`), IP addresses are allocated deterministically:

| IP Address | Role & Service | Operational Function |
| :--- | :--- | :--- |
| **`10.240.0.1`** | **Docker Bridge Gateway** | Host kernel gateway interface (`br-xxx`). |
| **`10.240.0.2`** | **`oops` Container (Core Daemon)** | **Fixed Well-Known DNS Server (Port 53) + Webhook Orchestrator**. Always first container IP. |
| **`10.240.1.0/24`** | Stack Network 1 (`oops_default`) | Application containers (`api`, `web`, `gateway`). |
| **`10.240.2.0/24`** | Stack Network 2 (`oops_db`) | Database containers (`mysql`, `postgres`, `redis`). |
| **`10.240.3.0/24`+** | Additional Stack Networks | Additional modular compose stacks (`storage`, `tools`, etc.). |

#### Core Pillars of `network.subnet`:

`network.subnet` มีบทบาทหลัก 2 ประการที่สำคัญต่อระบบทั้งฝั่ง Server และฝั่ง Dev Client:

1. **Server-Side: Compose Network Validator (Strict CIDR Containment)**
   - ทุก Network ที่ถูกประกาศใน `compose.yaml` (รวมถึง modular stacks ทุกตัว) **ต้องอยู่ภายใต้ Subnet นี้ (Subnet Containment)**
   - ตัวอย่างเช่น หากกำหนด `network.subnet: 10.240.0.0/16`:
     - ใน `compose.yaml` ประกาศ `10.240.1.0/24`, `10.240.2.0/24` -> **ผ่าน (Valid)**
     - หากมีใครไปประกาศ `172.20.0.0/16` หรือ `10.241.1.0/24` -> คำสั่ง `oops up` หรือ `oops deploy` จะ **Reject / Fail-Fast ทันที** เพื่อป้องกันไม่ให้มี Network หลุดออกนอกขอบเขต Routing Boundary
2. **Client-Side (Dev): Route Authority & VPN Gateway Target**
   - เมื่อเครื่อง Dev รัน `oops tunnel @<remote> --mesh` ตัว Client จะดึงค่า `network.subnet` นี้มาเป็น **Autoritative Routing Rule บนเครื่อง Dev**
   - สั่ง OS Routing Table บนเครื่อง Dev โดยตรง: **"Traffic ทุกแพ็กเก็ตที่มุ่งหน้าไปยังช่วง Subnet นี้ (เช่น `10.240.0.0/16`) ให้ Route วิ่งตรงเข้า VPN Gateway Interface (Tunnel Interface)"**
   - ไม่ต้อง Route กว้างเกินความจำเป็น (ไม่แตะ `0.0.0.0/0`) และไม่ชนกับ IP วงอื่นบนเครื่อง Dev
3. **Pre-flight Conflict Guard**: ตรวจสอบก่อนสร้าง Route ว่า `network.subnet` ไม่ชนกับ Physical Interface (เช่น WiFi หรือ VPN บริษัท) บนเครื่อง Dev
4. **Zero-Config on Client**: ใน `remote.yaml` บนเครื่อง Dev ไม่ต้องพิมพ์ Subnet เอง Client จะ Handshake และดึงค่ามาตั้ง Route ให้โดยอัตโนมัติ

### Routing Topology
Client workstations maintain a single, clean `/16` CIDR route per connected remote:
```bash
# Traffic for @b9tpe (Node 240) routed into b9tpe tunnel interface
10.240.0.0/16 -> oops-tunnel-b9tpe

# Traffic for an office host (Node 50) routed into office host tunnel interface
10.50.0.0/16 -> oops-tunnel-office50
```

The host OS kernel routes packets strictly by Octet 2 (`Node ID`), guaranteeing zero cross-talk or route ambiguity.

### Unified Single-TLD Architecture (`<service>.<remote>`)

To eliminate DNS complexity, proxy mismatches, and multi-domain translation layers, we establish the **Remote Identity Invariant**:

$$\mathbf{TLD} \equiv \langle\text{Remote Name}\rangle$$

Every Oopsbox machine maintains a single, authoritative TLD representing its environment namespace:
- **Local Dev Workstation**: `dns.tld: oops` $\rightarrow$ services are `<service>.oops` (e.g. `mysql.oops`, `web.oops`)
- **Remote Cloud VM (`@b9tpe`)**: `dns.tld: b9tpe` $\rightarrow$ services are `<service>.b9tpe` (e.g. `mysql.b9tpe`, `redis.b9tpe`)
- **Office / Lab Server (`@anthole`)**: `dns.tld: anthole` $\rightarrow$ services are `<service>.anthole` (e.g. `httpbin.anthole`, `grafana.anthole`)

$$\text{Domain} = \langle\text{service}\rangle \;.\; \langle\text{remote}\rangle$$

#### Container Hostname Registration Rules in `oops dns`:
1. **Automatic Baseline (Zero-Config)**:
   - For every running container, `oops dns` automatically registers its compose service name with the active TLD:
     $$\langle\text{service}\rangle \;.\; \langle\text{tld}\rangle$$
     Developers never have to write boilerplate `hostname:` in `compose.yaml` for standard services.
2. **Explicit Custom Hostname (Co-existence & Wildcards)**:
   - If a container explicitly specifies `hostname:` (or label `oops.dns.hostname`), `oops dns` registers that hostname alongside the automatic `<service>.<tld>`:
     - Exact domain: `hostname: mail.test` $\rightarrow$ registered.
     - Wildcard domain: `hostname: .web.b9tpe` $\rightarrow$ matches `web.b9tpe` and all subdomains `*.web.b9tpe`.
   - Both the automatic service name and custom hostnames resolve to the same container IP.

#### macOS Resolver Hygiene & Lifecycle Management:
- **One-Time Sudo Setup**:
  - When `oops remote connect @b9tpe` is executed for the first time, the CLI creates `/etc/resolver/b9tpe` with `nameserver <oops_container_ip>` (retrieved via `GetOopsContainerIP()`, identical to `/etc/resolver/oops`, e.g. `192.168.97.3`), pointing directly to the local `oops` DNS container on port 53.
- **Zero Sudo in Daily Workflow**:
  - During daily operations (`oops tunnel up @b9tpe` or `oops tunnel down @b9tpe`), **zero sudo prompts are required**.
  - When the tunnel is active, Local `oops dns` forwards queries for `*.b9tpe` directly to the remote DNS or local port loopback.
  - When the tunnel is inactive, Local `oops dns` instantly responds with `NXDOMAIN` (0ms fail-fast).
- **Automated Lifecycle Hooks**:
  - `oops remote remove @<remote>`: Automatically cleans up `/etc/resolver/<remote>` when deleting a remote capsule.
  - `oops remote rename @<old> @<new>`: Automatically renames `/etc/resolver/<old>` to `/etc/resolver/<new>`.
- **All-in-One Health & Repair via `oops doctor`**:
  - `oops doctor` acts as the single unified diagnostic and self-healing tool project-wide.
  - Scans `/etc/resolver/` on macOS:
    - Flags missing resolver files for registered remotes (and re-creates them).
    - Flags orphaned/stale resolver files that have no matching remote or capsule.
  - Passing `--repair` (e.g. `oops doctor --repair`) automatically cleans up orphaned resolver files and repairs directories in one single pass.

#### Dual-Mode Resolution Behavior:

| Mode | Target Service | Resolution Path | Resolved IP | Traffic Route |
| :--- | :--- | :--- | :--- | :--- |
| **Mesh Mode** (`--mesh`) | `mysql.b9tpe:3306`<br>`grafana.b9tpe:3000` | Local DNS forwards query `mysql.b9tpe` to remote DNS at `10.240.0.2:53` | Remote Container IP `10.240.2.5` | Direct L3 VPN interface routing to container port |
| **Port Mode** (`--port`) | `mysql.b9tpe:3306` | Local DNS resolves `mysql.b9tpe` directly from local forwarding table | Local Loopback IP (e.g. `127.240.0.1`) | SSH `-L` local forwarded port |
| **Office LAN / MikroTik** | `httpbin.anthole:80` | MikroTik RouterOS DNS forwards `*.anthole` to `10.69.69.50:53` | Container IP `10.50.1.2` | Native Gigabit LAN direct routing |

#### Benefits:
- **Zero DNS Engine Bloat**: No query string mutation, no reverse response translation, 1 table lookup.
- **Zero Caddy 404 Discrepancy**: If Caddy is used, it natively binds to `.<remote>` because `tld == <remote>`.
- **Pure Dev Ergonomics**: Developers connect directly to `<service>.<remote>:<port>` in TablePlus, DBeaver, Postman, or browsers.
- **Zero Public Leakage**: Private TLD queries (`*.b9tpe`, `*.anthole`, `*.oops`) never forward to public upstream DNS (`1.1.1.1` / `8.8.8.8`).

### Subnet Collision Analysis & Why `10.240+` is the Winning Space

Empirical verification of alternative private IP spaces:
1. `172.16.0.0/12` (RFC 1918 Class B):
   - **Rejected**: Octet 2 is legally bounded to `16 - 31` (numbers outside 16-31 are public internet IPs). Within 16-31, Docker bridges actively consume `172.17` through `172.24` (empirically confirmed on `@b9tpe`), and AWS VPC default uses `172.31`. There is insufficient space for Node IDs.
2. `127.0.0.0/8` (Loopback):
   - **Rejected**: Tested live with Docker daemon (`docker network create --subnet 127.2.0.0/16 test-127`). Linux kernel `rt_is_loopback` fails with `route for the gateway 127.2.0.1 could not be found: invalid argument`. Loopback addresses cannot be assigned as gateways for network bridges.
3. `100.64.0.0/10` (CGNAT RFC 6598):
   - **Rejected**: Cloudflare WARP / Zero Trust actively uses `100.96.0.0/12`, and Tailscale allocates `100.64.0.0/10`. Causes direct collision when developers use Cloudflare or Tailscale.
4. `10.0.0.0` - `10.206.255.255`:
   - **Rejected**:
     - `10.69.69.0/24`: Empirically confirmed as the active physical office WiFi/LAN subnet.
     - `10.128.0.0/9` (`10.128` to `10.206`): Reserved and actively assigned to Google Cloud VPC Auto-mode across all global regions (confirmed live on `@b9tpe` with Internal IP `10.140.0.16` in `asia-east1`).
5. `10.240.0.0/16` - `10.254.0.0/16` (**Selected Future-Proof Safe Zone**):
   - Far above GCP current maximum boundary (`10.206`), leaving a generous buffer of 34 `/16` blocks (`10.208` - `10.239`) for future Google Cloud region/datacenter expansions.
   - Completely clear of office WiFis, Docker default bridges, Cloudflare WARP, and Tailscale.
   - Provides 15 dedicated `/16` blocks (supporting 15 concurrent remote servers, each with 255 Docker networks and 253 containers).

---

## Hypotheses & Evaluated Options

- `[!]` **Option 1: Native SSH TUN (`ssh -w any:any`)**
  - *Mechanism*: Kernel-level TUN point-to-point interface over SSH.
  - *Evaluation / Rejection*: Requires `PermitTunnel yes` in `/etc/ssh/sshd_config` and `root` privileges on both local and remote hosts. Unviable on managed cloud VMs (GCP COS, locked down enterprise servers) where `sshd_config` is restricted.

- `[*]` **Option 2: SSH Dynamic SOCKS5 (`ssh -D`) + `tun2socks` Virtual Router (Primary Candidate)**
  - *Mechanism*: Establish a standard SSH Dynamic Port Forwarding proxy (SOCKS5), running completely in user-space over standard SSH and Google Cloud IAP. On the client, a lightweight virtual TUN adapter running `tun2socks` (inside the existing `oops-tunnel-<remote>` container or host daemon) captures `10.<node_id>.0.0/16` and pipes TCP/UDP streams through SOCKS5 to the remote host.
  - *Trade-offs*: High compatibility, zero server-side setup, works over IAP, no root requirement on remote servers.

- `[ ]` **Option 3: User-Space WireGuard over SSH Streaming Tunnel**
  - *Mechanism*: Run user-space WireGuard (e.g. `wireguard-go`) on the remote server streamed through an SSH tunnel channel.
  - *Trade-offs*: Extremely high performance and native UDP handling, but requires staging a WireGuard binary/container on the remote host.

---

## Workspace & Compose Health Check (`oops doctor`)

To ensure absolute system stability and prevent misconfigurations from reaching production, we introduce the first-class `oops doctor` command.

### Command Usage Matrix

| Command | Scope | Target & Verification Items |
| :--- | :--- | :--- |
| **`oops doctor`** | Full Audit | Comprehensive health check across Compose, DNS, Directory Hierarchy, and Docker Daemon. |
| **`oops doctor compose`** | Local Compose | Audits local `compose.yaml` and `oops.yaml` for subnet containment, IPAM, and ports. |
| **`oops doctor compose @<remote>`** | Single Remote | Audits remote capsule `/@<remote>/compose.yaml` against its `network.subnet`. |
| **`oops doctor compose --all`** | Global Compose Sweep | Sweeps and validates all compose files across Local workspace and all registered remote capsules (`/@b9tpe`, `/@anthole`, ...). |
| **`oops doctor dns`** | DNS Health | Audits local resolver, macOS `/etc/resolver/` entries, TLD alignment (`tld == remote`), and active records. |
| **`oops dns prune`** | DNS Action | Prunes orphaned/stale `/etc/resolver/` files on macOS detected by `oops doctor dns`. |

### Diagnostic Verification Checklist
1. **Subnet Containment & IPAM Compliance**:
   - Reads `x-oops.network.subnet` (e.g. `10.240.0.0/16`).
   - Verifies that every network declared in `compose.yaml` (and modular stack files) has an IPAM subnet that is a valid subset of `network.subnet`.
   - Alerts if any network has unconstrained auto-assigned IP or conflicts with gateway (`.1`) and DNS (`.2`).
2. **Infrastructure IP Reservation**:
   - Ensures `10.<node>.0.1` (Gateway) and `10.<node>.0.2` (`oops` core DNS daemon) are reserved and unassigned to user containers.
3. **Capsule & Directory Hierarchy**:
   - Verifies standard directories (`config/`, `data/`, `backups/`) exist and have proper permissions.
4. **Modular Stack Resolution (`x-oops-stack`)**:
   - Verifies all declared stack files (e.g., `- /edge`, `- /db`) exist on disk and parse cleanly.
5. **Port Clash & Domain Uniqueness**:
   - Checks for duplicate port bindings or clashing service domain labels.
6. **macOS Resolver Health & Hygiene (`/etc/resolver/`)**:
   - Verifies all registered remotes have corresponding `/etc/resolver/<remote>` files.
   - Detects orphaned/stale resolver files that have no matching remote or capsule.
   - When running `oops doctor --repair`, automatically cleans orphaned resolvers and generates missing ones.

---

## Discussion & Trade-offs

### 2026-10-24
- Evaluated limitation of current `-L` port-forwarding: unable to scale to dozens of microservices or arbitrary container endpoints.
- Evaluated DNS architectures: Unified `.oops` TLD vs Distinct `.<remote>` TLD vs Portal Namespace.
  - **Decided on Single-TLD Remote Identity Architecture (`<service>.<remote>`)**:
    - Established the **Remote Identity Invariant**: `dns.tld == <remote_name>` (e.g. `b9tpe`, `anthole`, `oops`).
    - Eliminates DNS engine bloat, query mutation, and reverse response patching.
    - **Automatic Service Registration**: `oops dns` automatically registers `<service>.<tld>` for every container without requiring `hostname:` boilerplate in `compose.yaml`.
    - **Custom Hostname Support**: If `hostname:` or `oops.dns.hostname` is defined, it is registered alongside `<service>.<tld>` (including wildcards like `*.web.<remote>`).
    - **One-Time Sudo Setup**: Mac resolver `/etc/resolver/<remote>` is created once during `oops remote connect @<remote>` pointing to local `oops` container IP (via `GetOopsContainerIP()`, identical to `/etc/resolver/oops`). Daily `oops tunnel up/down` operations require **zero sudo prompts**.
    - **Caddy Alignment**: Because `tld == <remote>`, Caddy virtual hosts natively match incoming `Host` headers without 404 errors.
- Evaluated and tested IPAM address spaces:
  - `172.16.0.0/12`: Inadequate Octet 2 range (16-31) and heavily overlaps with Docker bridges (`172.17`-`172.23`) and AWS VPC (`172.31`).
  - `127.0.0.0/8`: Blocked by Linux kernel (`rt_is_loopback`) when creating Docker bridge networks.
  - `100.64.0.0/10` (CGNAT): Overlaps with Cloudflare WARP (`100.96.0.0/12`) and Tailscale.
  - `10.0` - `10.206`: Overlaps with enterprise office LAN (`10.69.69.0/24`) and Google Cloud VPC Auto-mode (`10.128` - `10.206`, empirically verified on `@b9tpe` at `10.140.0.16`).
- **Final Agreement**: Adopted **`10.240.0.0/16` - `10.254.0.0/16`** (`Node ID`: `240` to `254`) as the future-proof, collision-free safe zone for Oops Mesh Network. Leaving a 34-block buffer (`10.208` - `10.239`) for future Google Cloud datacenter expansions.
- **Eliminated Artificial `node_id` in `remote.yaml`**: Recognized that `remote.yaml` should remain lean (`host`, `path`, `tunnel.mode`). The authoritative subnet belongs to the server itself in `oops.yaml` under `x-oops.network.subnet: 10.240.0.0/16`. The client tunnel automatically discovers this subnet during handshake without requiring manual `node_id` registration.
- **Fixed Infrastructure IP Policy**: Declared that `oops` core daemon container (DNS + Webhook) is always assigned the first container IP `10.<node>.0.2` (with `.1` reserved for bridge gateway), guaranteeing immutable, deterministic DNS server addressing across restarts and tunnels.
- **Docker Subnet Control & Google Cloud COS Invariant**:
  - Evaluated modifying remote Docker daemon via `/etc/docker/daemon.json` (`default-address-pools`): **Rejected as a runtime automation mechanism** because Google Cloud Container-Optimized OS (COS) uses a read-only root filesystem where `/etc/docker/` cannot be modified or reloaded at runtime via SSH without `cloud-init` during instance provisioning.
  - Concluded that **Virtual Overlay / VIP Mapping (Approach 3)** or **Compose-level IPAM (Approach 1)** are the only cross-platform architectures that remain completely turnkey across Google Cloud COS, Debian/Ubuntu, and edge hardware (Raspberry Pi) without requiring host OS reconfiguration.

---

## Open Questions

- [ ] **Remote Network Mapping & NAT**: When packets arrive at the remote server via SOCKS5, how does the remote side route to `10.<node_id>.<net_id>.<container_id>`? Does SOCKS5 resolve by container name/IP, or does a remote container proxy rewrite the IP?
- [ ] **macOS Host TUN Interface**: On macOS (Darwin), creating a host `utun` interface without full VPN apps usually requires `sudo ifconfig utun...` or NetworkExtension. Can we achieve this inside Docker container network namespace without prompting for root on host?
- [x] **Subnet Dynamic Discovery**: When client runs `oops tunnel @<remote>`, the client tunnel automatically discovers `network.subnet` from the remote `oops.yaml` via SSH handshake, keeping `remote.yaml` lean and zero-config.
- [x] **Single-TLD Identity & Auto-Registration**: Client resolves `<service>.<remote>` via `/etc/resolver/<remote>`, remote DNS registers `<service>.<tld>` + custom `hostname` automatically.

---

## Next Steps

- [ ] Create a minimal proof-of-concept (PoC) script testing `tun2socks` with SSH `-D` SOCKS5 proxy to verify latency and DNS resolution.
- [ ] Test container routing via remote SOCKS5 vs remote TUN endpoint.
- [ ] Discuss and decide between client-side DNS rewrite vs SOCKS5 domain proxying.

