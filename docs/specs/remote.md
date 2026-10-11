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

### 2. Remote Server Connection & First-Time Sync (`oops remote connect @<remote> [remote-path]`)

#### Target Naming & SSH Host Parity
- **Argument Format**: Accepts `@<host>` or `user@<host>` (e.g. `@prod`, `@vps-1`, `root@192.168.1.50`).
- **Capsule Name Extraction**: Local capsule directory name always normalizes to `@<host>` (e.g. `root@192.168.1.50` -> Capsule `/@192.168.1.50/`).
- **Host Collision Resolution Invariant**: If multiple remote environments share the same IP/host (e.g. different users, SSH ports, or keys), operators MUST declare named host entries in `~/.ssh/config` (e.g. `Host prod-api`, `Host staging-db`), and invoke `oops remote connect @prod-api`.

#### First-Time Detection & Sync State Machine [โฟลว์ตรวจสอบและซิงก์ครั้งแรก]
When executing `oops remote connect @<remote> [remote-path]` (default path: `~/oopsbox`):

```text
                  ┌────────────────────────────────────────┐
                  │ oops remote connect @<remote> [path]   │
                  └──────────────────┬─────────────────────┘
                                     │
                    (ตรวจ Server และ Local Capsule)
                    (Inspect Remote Server & Local Capsule)
                                     ▼
                ┌─────────────────────────────────────────┐
                │ Server มี ~/oopsbox?                    │
                │ Local มี /@<host>/ หรือยัง?             │
                │ [Server has workspace / Local capsule?] │
                └────────────────────┬────────────────────┘
                                     │
         ┌───────────────────────────┼───────────────────────────┐
         ▼                           ▼                           ▼
  [มีทั้ง 2 ฝั่ง]             [มีเฉพาะ Server]            [มีเฉพาะ Local]
  [Both Sides Exist]          [Server Only]               [Local Only]
         │                           │                           │
  ถามผู้ใช้ทันที                 Auto PULL                   Auto PUSH
  [push / pull]               มาที่ /@<host>/              ไปที่ Server
  [Prompt push/pull]          [Only to Capsule]           [To Server]
```

- **Conflict Resolution (Both Exist) [มีทั้งสองฝั่ง]**: If both remote workspace and local capsule contain configuration files, prompts operator:
  `Choose sync direction [push/pull] (default: pull):`
- **Server-First (Remote Only) [มีเฉพาะ Server]**: If server already runs an Oopsbox workspace but local capsule is fresh, automatically triggers **Pull** to initialize `/@<host>/`.
- **Local-First (Local Only) [มีเฉพาะ Local]**: If local capsule already contains configurations but remote server is fresh, automatically triggers **Push** to prepare remote server.
- **Fresh Setup (Neither Has Files) [ไม่มีทั้งสองฝั่ง]**: Provisions empty `/@<host>/remote.yaml` ready for `oops deploy @<host>`.

#### Capsule Isolation Invariant (Safety Guard) [ความปลอดภัยการแยกแคปซูล]
- **Strict Isolation**: When pulling from remote server, files (`.env`, `config/env/*.env`, `config/secrets/*`) MUST be written exclusively inside **`/@<host>/`**.
- **Root Workspace Protection**: Remote files must NEVER be pulled or unpacked into the root workspace directory, preventing corruption or accidental overwriting of local development stacks.

- **Connection Manifest (`/@<host>/remote.yaml`)**:
  ```yaml
  host: user@192.168.1.50 # SSH connection target or ~/.ssh/config alias
  path: ~/oopsbox         # Remote working directory
  ```

### 3. Remote Server Management Commands
- **List Remotes (`oops remote list`)**: Discovers and prints all registered environments by reading `/@*/remote.yaml`.
- **Remove Remote (`oops remote remove @<remote>`)**: Deletes the local `/@<remote>/` directory.
- **Remote Execute (`oops rx [@<remote>] <cmd>`)**: Executes commands on the remote server inside the target directory via SSH.

### 4. 2-Step Direct SSH Streaming Pipeline (`oops deploy`)
When executing `oops deploy [/<project>] [@<remote>]`:
- **Step 1 — Base Manifest Streaming**: Packs base manifests (`compose.yaml`, `services/`, `config/`), excluding all `*.local.yaml` and all `/@*` directories, and streams via `tar` over SSH to unpack at `path`.
- **Step 2 — Overlay Application**: Streams the contents of `/@<remote>/` (excluding `remote.yaml`) directly over the target directory on the remote server.
- **Step 3 — Container Execution**: Invokes `docker compose up -d` remotely via SSH.

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops remote connect` | `@<remote> [path] [--path <p>]` | Connects to remote host, detects workspace state, syncs capsule, writes `/@<host>/remote.yaml` |
| `oops remote list` | *(none)* | Discovers and lists all registered remotes |
| `oops remote remove` | `@<remote>` | Removes registered remote capsule directory `/@<remote>/` |
| `oops deploy` | `[/<project>] [@<remote>]` | Deploys stack to remote server via Direct SSH Tar Streaming |
| `oops rx` | `[@<remote>] <command>` | Forwards command execution to remote server over SSH |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops remote`, `oops deploy`, `oops rx`).
- **Downstream Dependencies**: Native SSH client (`ssh`, `scp`), remote Docker daemon.
- **Bounded Blast Radius**:
  - `cmd/remote.go`: CLI commands for remote server management.
  - `cmd/deploy.go`: CLI command for streaming deployment.
  - `internal/remote/`: Connection resolver, remote config loader, SSH streamer.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./cmd/... ./internal/remote/...`
- **Acceptance Scenarios**:
  - `oops remote connect @prod` connects to remote, verifies state, and creates `/@prod/remote.yaml` with valid YAML formatting.
  - `oops remote list` accurately lists all directories matching `/@*/remote.yaml`.
  - `oops deploy @prod` accurately identifies the remote target and streams base + overlay manifests excluding `remote.yaml`.
