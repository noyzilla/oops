---
title: Oops Remote Server Management & Direct SSH Deploy
status: active
tags: [oops-remote, remote-sync, ssh, direct-deploy, docker-compose, overlay]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/cli.md", "docs/specs/env-sync.md"]
---

# Specification: Oops Remote Server Management & Direct SSH Deploy

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/oopsbox.md](oopsbox.md), [docs/specs/cli.md](cli.md), [docs/specs/env-sync.md](env-sync.md)

## Overview & Scope

This specification defines the `oops remote` and `oops deploy` subsystem. It replaces server-side Git dependencies (bare repositories, hooks, and Git remotes) with a **Zero-Git Direct SSH Streaming Model**. Remote targets are managed locally as self-contained capsules (`/@<remote>/`), and deployments stream configuration manifests and directory overlays directly over SSH.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Remote Host (`@<remote>`)**: Remote server target registered locally inside `/@<remote>/remote.yaml`.
- **Remote Capsule (`/@<remote>/`)**: Directory containing all server-specific manifests (`remote.yaml`, `compose.yaml`, `.env`, and `config/`).
- **Direct SSH Streaming**: Tarball archive streaming over SSH connection directly into the remote working directory.
- **Remote Workspace Path**: The working directory on the server specified in `remote.yaml` (default: `~/oopsbox`).

## Business Rules & Logic Invariants

### 1. Zero-Git Server Architecture
- **No Git on Server**: Remote hosts require only `ssh` and `docker`. No Git installation, Git Bare repository, or Git push hook is required on the server.
- **No Git Remote Configuration**: The CLI does not register Git remotes in `.git/config`. Remote targets are discovered dynamically by scanning `/@*/remote.yaml`.
- **Pure Logical Naming (No `oops-` Prefix)**: Remote targets use clean logical identifiers directly (e.g. `@prod`, `@staging`, `@vps-1`) without legacy Git namespace prefixes.

### 2. Remote Server Connection & Blueprint Initialization (`oops remote connect @<remote>[:<path>]`)

#### Target Naming & SCP-Style Path Resolution [ไวยากรณ์ระบุ Target และ Remote Path]
- **Argument Format**: Accepts `@<host>[:<path>]` or `user@<host>[:<path>]` (e.g. `@prod`, `@prod:~/oopsbox`, `root@192.168.1.50:/opt/oopsbox`).
- **Capsule Name Extraction**: Local capsule directory name always normalizes to `@<host>` (e.g. `root@192.168.1.50:/opt/oopsbox` -> Capsule `/@192.168.1.50/`).
- **SCP/Rsync Colon Path Convention [มาตรฐาน Path ต่อท้ายโคลอน]**:
  - **Omitted (ไม่มี `:`)**: Defaults to `~/oopsbox` in operator's home directory.
  - **Explicit Home (`:~/path`)**: Resolves relative to operator's home directory (e.g. `@prod:~/custom` -> `~/custom`).
  - **Relative to Home (`:path`)**: Unprefixed path is automatically interpreted relative to home (e.g. `@prod:oopsbox` -> `~/oopsbox`).
  - **Absolute Root (`:/path`)**: Starts with `/` for system-wide root directories (e.g. `root@server:/opt/oopsbox` -> `/opt/oopsbox`).
- **Host Collision Resolution Invariant**: If multiple remote environments share the same IP/host (e.g. different users, SSH ports, or keys), operators MUST declare named host entries in `~/.ssh/config` (e.g. `Host prod-api`, `Host staging-db`), and invoke `oops remote connect @prod-api`.

#### First-Time Detection & Blueprint Initialization State Machine [โฟลว์ตรวจสอบและสร้างค่าเริ่มต้นจาก Blueprint]
When executing `oops remote connect @<remote>[:<path>]`:

- **Connection Manifest (`/@<host>/remote.yaml`)**:
  ```yaml
  host: user@192.168.1.50 # SSH connection target or ~/.ssh/config alias
  path: ~/oopsbox         # Remote working directory
  ```
- **Blueprint-Driven Initialization [การสร้างค่าเริ่มต้นเมื่อเชื่อมต่อเครื่องใหม่]**:
  - If remote server already contains `compose.yaml`, `oops.yaml`, or `.oopsallow`: Automatically pulls them down into `/@<host>/` following `.oopsallow`.
  - If remote server is fresh (no manifests): Generates capsule manifests from workspace blueprint templates:
    - `/@<host>/compose.yaml` initialized from `compose.yaml.example` (clean shared network base).
    - `/@<host>/oops.yaml` initialized from `oops.yaml.example` (daemon orchestrator compose manifest).
    - `/@<host>/.oopsallow` initialized from `.oopsallow.example` (bidirectional sync whitelist).
    - `/@<host>/.env` initialized from `.env.example` with cryptographically secure random `OOPS_SECRET` and `OOPS_DB_*_PASSWORD` credentials.
    - `/@<host>/config/` initialized for host-specific configuration overlays.
- **Remote Server Oopsbox Provisioning**:
  - Automatically ensures standard Oopsbox directory hierarchy exists on the remote host (`<remotePath>/{services,config,data,backups}`).
  - When connecting to a fresh remote host (`!remoteHasWorkspace`), performs initial streaming delivery of required service templates and capsule overlay manifests directly to the remote server so the Oopsbox workspace is fully initialized and operational immediately upon connection.
- **Capsule Isolation Invariant**: All remote host manifests (`.env`, `compose.yaml`, `oops.yaml`, `compose*.local.yaml`, `config/`) remain strictly inside `/@<host>/` and never mutate root workspace files.

#### Automated Prerequisite Bootstrap [การติดตั้ง Prerequisite อัตโนมัติ]
Before workspace detection and sync, `oops remote connect` automatically inspects whether `docker compose` and `oops` CLI are installed on the remote server:
- If missing, automatically installs `docker-compose-plugin` (supporting COS, Debian/Ubuntu `apt-get`, and CentOS/RHEL `yum/dnf`) and downloads the latest `oops` CLI binary into `/usr/local/bin/oops` (or `/var/lib/google/bin/oops` on COS).
- Operators can also run `oops remote setup [@<remote>]` at any time to explicitly install or force-upgrade `oops` CLI and `docker compose` to their latest releases.

### 3. Symmetric Git/Remote Synchronization Model

Oops adopts a clean, bidirectional synchronization model modeled after Git remotes, governing outgoing and incoming file transfers via `/@<remote>/.oopsallow`:

```text
Local Capsule (/@<remote>/) ───[oops remote push / oops deploy]───> Remote Server (~/oopsbox/)
Local Capsule (/@<remote>/) <───[oops remote pull]────────────────── Remote Server (~/oopsbox/)
Container Execution         ───[oops rx up / down / restart]──────> Remote Docker Daemon
```

#### Outgoing Delivery: `oops remote push` *(Alias: `oops deploy`)*
Pure streaming delivery of application manifests and capsule overrides. **No automatic container startup**:
- **Target Specification (`[/<stack1>,/<stack2>] [@<remote>]`)**:
  - Operators can target a single stack (`/app`) or a cluster of stacks using comma-separated notation (`/app,/db`).
  - Order-agnostic: `@<remote>` can be specified before or after target stacks.
- **Strict Pre-Flight Dependency Guard (Zero-Accidental Leak Invariant)**:
  - If a targeted stack depends on another stack (e.g. `/app` references `/db/mysql` or `x-oops-depends: [db]`):
    - **Remote Capsule Scope Precedence**: If `/db` is already declared in `/@<remote>/compose.db.yaml` (or within capsule base `compose.yaml`), the push proceeds without error because `@<remote>` local scope already provides it.
    - **Explicit Inclusion Requirement**: If `/db` exists ONLY in the root workspace `compose.db.yaml` and is NOT in `/@<remote>/`, pushing `/app` alone is strictly rejected with Exit Code 1. The operator MUST explicitly include it (`/app,/db`) to confirm intention to ship the root stack manifest.
- **Step 1 — Base Manifest Streaming**:
  - Resolves required services via Dependency Resolution Algorithm (`x-oops-services:` + `depends_on:` in `compose.<stack>.yaml` and `/@<remote>/compose.yaml`, plus remote overrides `/@<remote>/compose.local.yaml` and `/@<remote>/compose.<stack>.local.yaml`).
  - **Local & Remote Precedence Invariant**: If a service is already declared under `services:` in the stack or within the `@remote` capsule (including `/@<remote>/services/*.yaml`), the algorithm strictly avoids pulling or streaming matching templates from workspace root `services/<service>.yaml` and `config/<service>/`.
  - Streams selected `compose.<stack>.yaml` (for stacks not overridden in `/@<remote>/`), resolved `services/<service>.yaml`, and matching `config/<service>/`.
  - Excludes local `./compose.yaml`, `./oops.yaml`, `oopsbox.yaml`, `*.local.yaml`, `/@*`, `.git`, `.scratch`, and `data/`.
- **Step 2 — Capsule Overlay Streaming (Filtered via `.oopsallow`)**:
  - Streams files in `/@<remote>/` that strictly match `/@<remote>/.oopsallow` (`compose.yaml` -> `~/oopsbox/compose.yaml`, `oops.yaml` -> `~/oopsbox/oops.yaml`, `.env`, `compose.local.yaml`, `compose.<stack>.local.yaml`, `compose.<stack>.yaml`, `config/*`).
- **Step 3 — Post-Delivery Prompt**:
  - Outputs summary of transferred manifests.
  - Prompts operator: `==> Push complete. Run 'oops rx [@<remote>] up [/<stack1>,/<stack2>]' to start or restart services.`

#### Incoming Synchronization: `oops remote pull`
Downloads configurations, overrides, and secrets from the remote server into the local capsule:
- **Default-Deny Protection (`.oopsallow`)**:
  - Filters remote files strictly using patterns defined in `/@<remote>/.oopsallow` (e.g. `.env`, `compose.yaml`, `oops.yaml`, `compose*.local.yaml`, `config/**`).
  - Hard-excludes `data/**`, `backups/**`, `*.log`, `*.sock`, `.git/**` to prevent accidental multi-gigabyte data dumps into local capsules.
- **Zero Local Mutation**: Writes exclusively into `/@<remote>/`. Root files remain untouched.

### 4. Remote Server Management & Execution Commands
- **List Remotes (`oops remote list`)**: Discovers and prints all registered environments by reading `/@*/remote.yaml`.
- **Remote Setup (`oops remote setup [@<remote>]`)**: Installs or upgrades `docker-compose-plugin` and `oops` CLI binary to the latest version on the remote server over SSH.
- **Remove Remote (`oops remote remove @<remote>`)**: Deletes the local `/@<remote>/` directory.
- **Remote Execute (`oops rx [@<remote>] <oops-cmd> [args...]`)**: Forwards and executes `oops` subcommands on the remote server inside the target workspace directory via SSH (e.g. `oops rx ps` -> executes `oops ps` on server).

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops remote connect` | `@<remote>[:<path>] [path] [--path <p>]` | Connects to remote host, auto-bootstraps environment, initializes capsule with blueprints, writes `/@<host>/remote.yaml` |
| `oops remote setup` | `[@<remote>]` | Installs or upgrades `docker-compose-plugin` and `oops` CLI to the latest version on the remote server |
| `oops remote push` | `[/<stack1>,/<stack2>] [@<remote>]` | Streams manifests and capsule overlay to remote server (filtered by `.oopsallow`; pure delivery, no auto-start) |
| `oops remote pull` | `[/<stack1>,/<stack2>] [@<remote>]` | Downloads remote configs and manifests into local capsule `/@<remote>/` (filtered by `.oopsallow`) |
| `oops deploy` | `[/<stack1>,/<stack2>] [@<remote>]` | Exact ergonomic alias for `oops remote push` |
| `oops remote list` | *(none)* | Discovers and lists all registered remotes |
| `oops remote remove` | `@<remote>` | Removes registered remote capsule directory `/@<remote>/` |
| `oops rx` | `[@<remote>] <oops-cmd> [args...]` | Forwards oops command over SSH to remote target (e.g. `oops rx up /crm`, `oops rx ps`) |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops remote`, `oops deploy`, `oops rx`).
- **Downstream Dependencies**: Native SSH client (`ssh`, `scp`), remote Docker daemon.
- **Bounded Blast Radius**:
  - `cmd/remote.go`: CLI commands for remote server management, `connect`, `push`, `pull`, `list`, `remove`.
  - `cmd/deploy.go`: CLI command for streaming deployment (delegates to push).
  - `cmd/rx.go`: CLI command for remote command forwarding.
  - `internal/remote/`: Connection resolver, remote config loader, SSH streamer.
  - `internal/sync/`: `.oopsallow` whitelist filter engine for bidirectional sync.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./cmd/... ./internal/remote/... ./internal/sync/...`
- **Acceptance Scenarios**:
  - `oops remote connect @prod` connects to remote, verifies state, and initializes `remote.yaml`, `compose.yaml`, `oops.yaml`, and `.oopsallow` from blueprints if fresh.
  - `oops remote list` accurately lists all directories matching `/@*/remote.yaml`.
  - `oops remote push @prod` / `oops deploy @prod` streams selective stack/services and capsule overlay without running `docker compose up -d`.
  - `oops remote pull @prod` downloads remote configs matching `.oopsallow` into `/@prod/` without downloading `data/` or `backups/`.
