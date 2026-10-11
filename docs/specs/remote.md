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

### 2. Remote Target Registration (`oops remote add <ssh-alias> [-r <name>]`)
- **Connection Manifest (`/@<name>/remote.yaml`)**:
  ```yaml
  host: oops-prod       # SSH host alias declared in ~/.ssh/config or user@ip
  path: ~/oopsbox       # Remote working directory
  ```
- **Automatic Directory Provisioning**: Running `oops remote add <ssh-alias> -r <name>` creates `/@<name>/` and writes `remote.yaml`.
- **Target Verification**: Pings remote host over SSH (`ssh <host> docker info`) to verify connectivity and Docker availability.

### 3. Remote Server Management Commands
- **List Remotes (`oops remote list`)**: Discovers and prints all registered environments by reading `/@*/remote.yaml`.
- **Remove Remote (`oops remote remove <name>`)**: Deletes the local `/@<name>/` directory.
- **Remote Execute (`oops rx [-r <name>] <cmd>`)**: Executes commands on the remote server inside the target directory via SSH.

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
| `oops remote add` | `<ssh-alias> [-r <name>]` | Registers a remote server creating `/@<name>/remote.yaml` |
| `oops remote list` | *(none)* | Discovers and lists all registered remotes |
| `oops remote remove` | `<name>` | Removes registered remote capsule directory `/@<name>/` |
| `oops deploy` | `[/<project>] [@<remote>]` | Deploys stack to remote server via Direct SSH Tar Streaming |
| `oops rx` | `[-r <remote>] <command>` | Forwards command execution to remote server over SSH |

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
  - `oops remote add prod-box -r prod` creates `/@prod/remote.yaml` with valid YAML formatting.
  - `oops remote list` accurately lists all directories matching `/@*/remote.yaml`.
  - `oops deploy @prod` accurately identifies the remote target and streams base + overlay manifests excluding `remote.yaml`.
