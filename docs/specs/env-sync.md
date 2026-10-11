---
title: Environment Variable & Secrets Synchronization
status: active
tags: [cli, ssh, env, secrets, deployment, docker, overlay]
synapses: ["ARCHITECTURE.md", "docs/specs/remote.md", "docs/specs/cli.md"]
---

# Specification: Environment & Secrets Synchronization

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Companion Spec**: [docs/specs/remote.md](remote.md), [docs/specs/cli.md](cli.md)

## Overview & Scope

Oops provides an isolated, out-of-band mechanism for synchronizing configuration (`.env` files) and sensitive data (`config/secrets/`) between the local developer workstation and remote servers via Native SSH.

Under the **1:1 Mirror Directory Sync Protocol**, environment configurations are encapsulated cleanly inside `/@<remote>/` (e.g. `/@prod/.env`). Filenames are never mangled, sliced, or suffixed with dot notations (`stripRemoteSuffix` is deprecated). Files mirror directly 1:1 with their remote destination paths.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Remote Overlay Capsule (`/@<remote>/`)**: The self-contained directory encapsulating all specialization files for a designated remote target.
- **Root Environment (`.env`)**: The machine-level environment variable file located at `/@<remote>/.env` locally and `.env` on the remote server.
- **Service Secrets**: Sensitive credential files located at `/@<remote>/config/secrets/*` locally and `config/secrets/*` on the remote server.
- **Out-of-Band Sync**: Secure transfer of credentials via SSH/SCP that bypasses version control entirely.

## Business Rules & Logic Invariants

### 1. The 1:1 Mirror Directory Sync Protocol
Filenames remain completely identical across local and remote environments:

| Target Resource | Local Path (Inside `/@<remote>/`) | Direction | Remote Path (Target Server Root) |
| :--- | :--- | :---: | :--- |
| **Root Environment** | `/@<remote>/.env` | ➡️ Push / ⬅️ Pull | `.env` |
| **Service Envs** | `/@<remote>/config/env/<service>.env` | ➡️ Push / ⬅️ Pull | `config/env/<service>.env` |
| **Service Secrets**| `/@<remote>/config/secrets/*` | ➡️ Push / ⬅️ Pull | `config/secrets/*` |
| **Service Configs**| `/@<remote>/config/<service>/*` | ➡️ Push / ⬅️ Pull | `config/<service>/*` |

- **Push Transformation**: Transfers files from `/@<remote>/` directly onto the remote target root without modifying filenames.
- **Pull Transformation**: Downloads files from the remote server directly into `/@<remote>/` with identical filenames.
- **Target Resolution**: Reads target SSH host and remote working directory from `/@<remote>/remote.yaml`.

### 2. Security & Git Hygiene Invariants

- **Zero Secrets in Git**: All `.env` files (including root `.env`, `compose.local.yaml`, and `/@<remote>/.env`) MUST be strictly Git-ignored.
- **Exclusion of `remote.yaml`**: The target connection manifest `/@<remote>/remote.yaml` is used exclusively for local orchestration and is strictly excluded from remote uploads.
- **Command Isolation**:
  - `oops sync env`: Synchronizes ONLY `.env` and `config/env/` files.
  - `oops sync secret`: Synchronizes ONLY `config/secrets/` files.
  - `oops sync`: Synchronizes both configurations and secrets.

---

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops sync push` | `[@<remote>]` | Uploads both environment files and secrets from `/@<remote>/` to remote server |
| `oops sync pull` | `[@<remote>]` | Downloads remote `.env` and secrets into local `/@<remote>/` |
| `oops sync env push` | `[@<remote>]` | Uploads ONLY environment files (`.env` and `config/env/`) |
| `oops sync env pull` | `[@<remote>]` | Downloads ONLY remote environment files |
| `oops sync secret push` | `[@<remote>]` | Uploads ONLY secret files (`config/secrets/`) |
| `oops sync secret pull` | `[@<remote>]` | Downloads ONLY secret files |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops sync`), deployment pipeline.
- **Downstream Dependencies**: Native SSH / SCP, remote filesystem permissions.
- **Bounded Blast Radius**:
  - `cmd/sync.go`: Cobra CLI commands for sync operations.
  - `internal/sync/`: 1:1 Mirror directory transfer orchestration.
  - `internal/remote/`: Resolution of target SSH connection from `/@<remote>/remote.yaml`.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./internal/sync/... ./cmd/...`
- **Acceptance Scenarios**:
  - Given local file `/@prod/.env`, running `oops sync push @prod` copies the file to `.env` on remote host.
  - Given remote file `config/secrets/db_pass.txt`, running `oops sync pull @prod` saves it to `/@prod/config/secrets/db_pass.txt`.
  - Non-matching remotes (e.g. running `@staging` when only `/@prod/` exists) produce actionable errors.
