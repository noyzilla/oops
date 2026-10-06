---
title: Deploy Key Management & Workspace Cloning
status: active
tags: [deploy-key, ssh, git, oopsbox, clone, cli]
synapses: ["CONTEXT.md", "docs/specs/cli.md", "docs/specs/oopsbox.md"]
---

# Specification: Deploy Key Management & Workspace Cloning [การจัดการ Deploy Key และการ Clone Workspace]

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Companion Specs**: [cli.md](cli.md), [oopsbox.md](oopsbox.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)

## Overview & Scope

This specification defines the `oops key` command suite for managing host-level SSH Deploy Keys (`ed25519`), along with the authentication helper integration in `oops box clone <repo> [path]`. It ensures automated, read-only authentication for Git-based workspace deployment without storing personal private keys on remote servers. [จัดการ Deploy Key แบบ ed25519 สำหรับ clone oopsbox โดยไม่ต้องวาง Private Key ส่วนตัวบน server]

Scope:
- **Key Management**: `oops key`, `oops key reset`, `oops key set`.
- **Workspace Cloning**: `oops box clone <repo> [path]` (defaults `path` to `.`).
- **Authentication Fallback**: Helpful CLI guidance, public key display, and direct Web URL generation for adding Deploy Keys on GitHub/GitLab.

## Domain Context & Ubiquitous Language

- **Deploy Key**: A dedicated, unpassphrased `ed25519` SSH keypair generated at `~/.oops/id_ed25519` used strictly for read-only Git operations.
- **Repository URL**: A SSH or HTTPS Git repository URL (e.g., `git@github.com:user/repo.git` or `https://github.com/user/repo.git`).
- **Direct Add Key URL**: Automatically derived web URL pointing directly to the Git host's Deploy Key creation page (e.g., `https://github.com/user/repo/settings/keys/new`).

## Business Rules & Logic Invariants

### 1. Key Management Commands (`oops key`)
- **Key Storage Location**: Deploy keys MUST be stored at `~/.oops/id_ed25519` (private key) and `~/.oops/id_ed25519.pub` (public key).
- **`oops key` (Show / Auto-Generate)**:
  - Checks if `~/.oops/id_ed25519.pub` exists.
  - If missing, automatically generates a new `ed25519` SSH keypair without a passphrase (`ssh-keygen -t ed25519 -f ~/.oops/id_ed25519 -N ""`).
  - Outputs the public key content to stdout with clear copy instructions.
- **`oops key reset`**:
  - Unconditionally regenerates a new `ed25519` keypair at `~/.oops/id_ed25519`, overwriting any existing key.
  - Outputs the newly generated public key.
- **`oops key set`**:
  - Interactively (or via stdin/args) prompts the user to paste an existing Private Key (and optional Public Key).
  - Saves the pasted key to `~/.oops/id_ed25519` with strict permissions (`0600`).

### 2. Workspace Cloning (`oops box clone <repo> [path]`)
- **Argument Defaults**: If `path` is omitted, it defaults to `.` (current working directory).
- **Git SSH Configuration**: Git operations invoked by `oops` MUST explicitly use `~/.oops/id_ed25519` (e.g., `GIT_SSH_COMMAND="ssh -i ~/.oops/id_ed25519 -o StrictHostKeyChecking=accept-new"`).
- **Authentication Failure Handling**:
  - If `git clone` fails due to authentication or permission errors:
    1. Display a friendly error message explaining that authentication failed.
    2. Automatically execute key check/generation (`oops key`) and print the Public Key.
    3. Parse `<repo>` to extract Git host, owner, and repository name.
    4. If the repository is on GitHub or GitLab, compute and display the direct URL to add the key:
       - **GitHub**: `https://github.com/<owner>/<repo>/settings/keys/new`
       - **GitLab**: `https://gitlab.com/<owner>/<repo>/-/settings/repository` (Deploy Keys section)
    5. Exit with non-zero status.

## Interface & Data Contracts

### CLI Syntax
```bash
oops key                  # Show public key (auto-generates if missing)
oops key reset            # Regenerate new ed25519 keypair
oops key set              # Set existing key from input

oops box clone <repo> [path] # Clone workspace repo (default path: .)
oops box pull [path]         # Git pull latest workspace updates using Deploy Key (default path: active box)
```

### Direct Add Key URL Resolution Algorithm
Given `<repo>`:
- `git@github.com:noyzilla/oopsbox.git` -> `https://github.com/noyzilla/oopsbox/settings/keys/new`
- `https://github.com/noyzilla/oopsbox.git` -> `https://github.com/noyzilla/oopsbox/settings/keys/new`
- `git@gitlab.com:group/project.git` -> `https://gitlab.com/group/project/-/settings/repository`

## Dependency & Blast-Radius Matrix

- **Upstream Callers (Inbound)**: `cmd/key.go`, `cmd/box.go`, `cmd/box_clone.go`.
- **Downstream Dependencies (Outbound)**: `ssh-keygen` or standard crypto/ssh library, `git` CLI, `internal/key` package.
- **Bounded Blast Radius**: `internal/key/`, `cmd/key.go`, `cmd/box.go`, `docs/specs/cli.md`.

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./internal/key/... ./cmd/...`
- **Happy Path**:
  - Running `oops key` on a clean machine creates `~/.oops/id_ed25519` and prints the public key.
  - Running `oops box clone <repo> <path>` clones using `~/.oops/id_ed25519`.
- **Error Handling**:
  - Cloning a private repo without access prints the public key and direct URL `https://github.com/owner/repo/settings/keys/new`.
