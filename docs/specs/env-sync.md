---
title: Environment Variable & Secrets Synchronization
status: active
tags: [cli, ssh, env, secrets, deployment, docker]
synapses: ["ARCHITECTURE.md", "docs/specs/remote.md", "docs/specs/cli.md"]
---

# Specification: Environment & Secrets Synchronization

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Companion Spec**: [docs/specs/remote.md](remote.md)

## Overview & Scope

Oops provides an isolated mechanism for synchronizing configuration (`.env` files) and sensitive data (`secrets`) between the local developer workstation and remote servers via Native SSH.

Based on [Docker's Best Practices](https://docs.docker.com/compose/how-tos/environment-variables/best-practices/), sensitive information (passwords, API keys) **MUST NOT** be stored in environment variables. Instead, they must be managed via Docker Secrets. The Oops CLI supports both configurations by enforcing a strict **Suffix Mapping Protocol** during data transfer, and strictly separating the CLI commands (`oops env` vs `oops secret`) to prevent accidental exposure of sensitive data.

## Domain Context & Ubiquitous Language

- **ENV Sync**: The process of pushing or pulling environment variable configuration files via SSH/SCP.
- **Secrets Sync**: The process of pushing or pulling sensitive credential files via SSH/SCP.
- **Prefix/Suffix Mapping**: The bidirectional translation of file names during transfer to preserve `compose.yml` environment targeting without branching logic.
- **Root Config**: The global default environment variables located at `<oopsbox>/.env`.
- **Service Config**: The service-scoped environment variables located at `<oopsbox>/config/env/*.env`.
- **Service Secrets**: The service-scoped sensitive data located at `<oopsbox>/config/secrets/*`.

## Business Rules & Logic Invariants

### 1. The Suffix Mapping Protocol (Stateless Translation)

To ensure that the `compose.yml` file remains identical across all environments, the CLI translates filenames on the fly during transfer:

| Target | Local File Pattern (Git-Ignored) | Direction | Remote File Pattern (On Server) | Docker Native Usage |
| :--- | :--- | :---: | :--- | :--- |
| **Root Config** | `.env.<remote>` | ➡️ Push / ⬅️ Pull | `.env` | Compose auto-interpolation |
| **Service Config** | `config/env/<name>.<remote>.env` | ➡️ Push / ⬅️ Pull | `config/env/<name>.env` | `env_file: config/env/<name>.env` |
| **Service Secrets**| `config/secrets/<name>.<remote>.*` | ➡️ Push / ⬅️ Pull | `config/secrets/<name>.*` | `secrets: file: config/secrets/...` |

- **Push Transformation**: The `.<remote>` suffix is stripped from the filename when saving to the remote server.
- **Pull Transformation**: The `.<remote>` suffix is appended to the filename when saving to the local workstation.
- **1:1 Parity Invariant**: Because of suffix stripping, the remote `compose.yml` can statically mount configs and secrets without environment-specific conditional logic.

### 2. Best Practices & Security Invariants

- **Command Isolation**: `oops sync env` handles ONLY `.env` files. `oops sync secret` handles ONLY files in `config/secrets/`. This prevents developers from accidentally pushing secrets when they only intended to update a port mapping.
- **Separation of Concerns**: Non-sensitive configs (e.g., `PORT=8080`, `DEBUG=true`) are stored in `.env` files. Sensitive data (e.g., `DB_PASSWORD=supersecret`) are stored in raw text files inside `config/secrets/`.
- **Git Ignored**: All files matching `.<remote>` (e.g. `.env.prod`, `mysql.prod.env`, `db_pass.prod.txt`) MUST NOT be committed to Git.
- **Strict Matching**: Only exact string matches of the target remote name are synchronized. If a file is named `.env.staging` and the target is `prod`, it is strictly ignored during `oops sync push -r prod`.

## Interface & Data Contracts

### CLI Subcommands Specification

| Command | Arguments / Flags | Description |
| :--- | :--- | :--- |
| `oops sync push` | `[-r <remote>]` | Scans and uploads BOTH Configs (`.env`) and Secrets (`config/secrets/`) via SCP/SSH, stripping the suffix. |
| `oops sync pull` | `[-r <remote>]` | Downloads BOTH Configs (`.env`) and Secrets (`config/secrets/`) via SCP/SSH, appending the suffix locally. |
| `oops sync env push` | `[-r <remote>]` | Uploads ONLY Configs (`.env.<remote>` and `config/env/*.<remote>.env`). |
| `oops sync env pull` | `[-r <remote>]` | Downloads ONLY Configs. |
| `oops sync secret push` | `[-r <remote>]` | Uploads ONLY Secrets (`config/secrets/*.<remote>.*`). |
| `oops sync secret pull` | `[-r <remote>]` | Downloads ONLY Secrets. |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal.
- **Downstream Dependencies**: Native SSH/SCP binary or Go SSH Client, remote filesystem permissions.
- **Bounded Blast Radius**:
  - `cmd/sync.go`: CLI root for `oops sync push|pull|env|secret`.
  - `internal/ssh/`: SSH client extensions for SCP file transfer.
  - `internal/sync/`: Shared logic for suffix mapping and transfer orchestration.

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./cmd/... ./internal/ssh/... ./internal/sync/...`
- **Acceptance Scenarios**:
  - Given a local file `config/env/api.prod.env`, executing `oops sync env push -r prod` places the file at `config/env/api.env`.
  - Given a local file `config/secrets/db.prod.txt`, executing `oops sync secret push -r prod` places the file at `config/secrets/db.txt`.
  - Given a local file `config/secrets/db.prod.txt`, executing `oops sync env push -r prod` MUST ignore the secret file.
