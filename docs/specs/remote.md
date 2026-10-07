---
title: Oops Remote Server Management & Bare Sync
status: active
tags: [oops-remote, git-bare, remote-sync, ssh, bootstrap, docker-compose]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/deploy-key.md", "docs/specs/cli.md"]
---

# Specification: Oops Remote Server Management & Bare Sync

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Specs**: [docs/specs/oopsbox.md](oopsbox.md), [docs/specs/deploy-key.md](deploy-key.md), [docs/specs/cli.md](cli.md)

## Overview & Scope

This specification defines the `oops remote` command suite for remote server bootstrapping, Git Bare synchronization, and SSH-delegated remote orchestration. It strictly separates local workstation management (`oops box`) from remote server operations (`oops rx [-r <remote>] <command>` and `oops deploy [-r <remote>]`), allowing developers to sync, deploy, and inspect remote environments (e.g. production, staging) via Native SSH without third-party Git hosts.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Local Workstation (`oops box`)**: Local developer workstation environment (OrbStack / Colima / Docker Desktop).
- **Remote Host (`oops remote`)**: Remote server target registered via SSH alias in `~/.ssh/config` or `user@host`.
- **Git Bare Repository (`<bare-path>`)**: Fixed server-side bare Git repository at `~/.oops/oopsbox.git` serving as central sync point.
- **Remote Workspace Path (`<oopsbox-path>`)**: Fixed working tree directory at `~/oopsbox` on the server.
- **Explicit Deploy Execution**: Deployments are driven explicitly via `oops deploy [-r <remote>] [ref]`, pushing commit history and running SSH-delegated checkout and container up.

## Business Rules & Logic Invariants

### Workspace Requirement Invariant
- `oops remote add` and `oops deploy` MUST be executed within a valid local oopsbox workspace directory (`.git` or `stacks/` or `compose.yaml` present).
- If executed outside a valid oopsbox workspace directory, the command halts cleanly with:
  `Error: Not inside a valid oopsbox workspace directory`.

### Remote Server Registration (`oops remote add <ssh-target> [-r <remote-name>]`)
- **Git Remote Prefix Namespacing Invariant**:
  - Registered Git remotes in the local repository are strictly namespaced with `oops-` prefix (e.g. `oops-prod`, `oops-staging`).
  - Users interact exclusively with logical names (`prod`, `staging`) in all `oops` commands. The CLI automatically maps logical names to `oops-<name>` in Git.
- **Argument & Option Parsing**:
  - **Positional Argument**: `<ssh-target>` is strictly the first positional argument (e.g., `anthole`, `user@1.2.3.4`).
  - **Flag `-r, --remote <name>`**: Explicitly sets the logical remote name (e.g., `-r staging`).
- **Auto-Derivation Cascade (when `-r` is omitted)**:
  1. If logical `prod` (`oops-prod`) is not yet registered in workspace -> Default logical name is `prod`.
  2. If `prod` exists -> Extract `host` from `<ssh-target>`. If `oops-<host>` is available -> Default logical name is `host`.
  3. If `host` exists -> Fallback logical name is `<host>_<user>` (e.g., `1.2.3.4_ubuntu`).
  4. **Interactive Prompt**: If executed in an interactive TTY and `host` exists, prompt the user with a warning, using `<host>_<user>` as default pre-filled editable input.
- **Fixed Server Locations**:
  - Server Bare Repository is strictly fixed at `~/.oops/oopsbox.git`.
  - Server Workspace Directory is strictly fixed at `~/oopsbox`.
- **Git Remote Registration**: Registers local Git remote `oops-<name>` using standard SCP-style SSH notation: `<ssh-target>:.oops/oopsbox.git` (e.g., `anthole:.oops/oopsbox.git` or `captain@anthole.local:.oops/oopsbox.git`).
- **Google COS & Read-Only OS Support**:
  - Detects read-only filesystems (Google Container-Optimized OS / COS).
  - On Google COS, installs `oops` CLI into `/var/lib/google/bin/oops` and `docker-compose` plugin into `/var/lib/google/docker-cli-plugins/docker-compose`.
  - Configures `"cliPluginsExtraDirs": ["/var/lib/google/docker-cli-plugins"]` in `${HOME}/.docker/config.json` and `/root/.docker/config.json`.
- **Server Initialization**: Initializes `~/.oops/oopsbox.git` and `~/oopsbox` directly on the server via SSH bootstrap script.

### Remote Server Management (`oops remote rename <old-name> <new-name>`)
- Renames registered remote `<old-name>` to `<new-name>`.
- Executes `git remote rename oops-<old-name> oops-<new-name>` internally.

### Server-Side Bare Repository Initialization (`internal/remote/bootstrap.go`)
- **Directory Setup**: Creates `<bare-path>` as a bare Git repository (`git init --bare <bare-path>`).
- **Push Option Config**: Enables `git config receive.advertisePushOptions true` on the bare repository.
- **Post-Receive Hook**: Generates executable `hooks/post-receive` inside `<bare-path>`:
  ```sh
  #!/bin/sh
  OOPSBOX_DIR="~/oopsbox"
  case "$OOPSBOX_DIR" in
    \~/*) OOPSBOX_DIR="$HOME/${OOPSBOX_DIR#\~/}" ;;
    \~)   OOPSBOX_DIR="$HOME" ;;
  esac
  export GIT_WORK_TREE="$OOPSBOX_DIR"
  TARGET_REF=""
  DO_DEPLOY=0

  i=0
  while [ $i -lt ${GIT_PUSH_OPTION_COUNT:-0} ]; do
    eval "opt=\$GIT_PUSH_OPTION_$i"
    if [ "$opt" = "deploy" ] || [ "$opt" = "up" ]; then
      DO_DEPLOY=1
    fi
    i=$((i + 1))
  done

  if [ "$DO_DEPLOY" -ne 1 ]; then
    exit 0
  fi

  while read oldrev newrev refname; do
    case "$refname" in
      refs/tags/*)  TARGET_REF="tags/${refname#refs/tags/}" ;;
      refs/heads/*) TARGET_REF="${refname#refs/heads/}" ;;
    esac
  done

  if [ -n "$TARGET_REF" ]; then
    git checkout -f "$TARGET_REF"
    if command -v oops >/dev/null 2>&1; then
      oops up -C "$OOPSBOX_DIR" || true
    elif [ -f /var/lib/google/bin/oops ]; then
      /var/lib/google/bin/oops up -C "$OOPSBOX_DIR" || true
    fi
  fi
  ```

### Remote Deployment (`oops deploy [-r <remote>] [ref]`)
- **Top-Level Deployment Command**: `oops deploy` is the single, definitive command for deploying workspace updates to a remote server.
- **Argument Defaults**:
  - `-r <remote>`: Target logical remote server name (Default: `prod` mapped to `oops-prod`, or single registered `oops-*` remote if only one exists). Strictly uses `-r <remote>` to specify target remote server.
  - `[ref]`: Target Git reference to deploy (Default: active Git branch, e.g., `main`).
- **Execution Workflow**:
  1. Executes `git push <oops-remote> <ref>` to sync Git commits to remote `~/.oops/oopsbox.git`.
  2. Executes SSH to remote target:
     `git --git-dir=$HOME/.oops/oopsbox.git --work-tree=$HOME/oopsbox checkout -f <ref> && (oops up -C $HOME/oopsbox || /var/lib/google/bin/oops up -C $HOME/oopsbox)`

### Remote Command Execution (`oops rx [-r <remote>] <command> [args...]`)
- **Dedicated Subcommand**: `oops rx` (`rx` = Remote Execute) is the dedicated subcommand for executing developer operations directly on a remote server over SSH.
- **Universal Flag Standard (`-r` / `--remote`)**:
  - `-r <remote>` or `--remote <remote>` (or `--remote=<remote>`) is standardized project-wide as the single flag for specifying a remote server logical name.
  - Positional remote server name guessing across `oops rx` and `oops deploy` is strictly forbidden to prevent collision between remote names, references, and command names.
- **Remote Selection Priority**:
  1. Flag `-r <remote>` / `--remote` / `--remote=<remote>` (explicit).
  2. Fallback: Default remote `prod` (mapped to `oops-prod`) or single registered `oops-*` remote if exactly one exists.
- **Execution Workflow**:
  - `oops rx [-r <remote>] <command> [args...]` executes over SSH:
    `ssh -t <ssh-target> "cd ~/oopsbox && (oops <command> [args...] || /var/lib/google/bin/oops <command> [args...])"`
  - Allocates pseudo-TTY (`-t`) when running interactively to preserve ANSI colors, tailing output, and signal handling.
- **Excluded Commands**:
  - `remote`, `deploy`: Forbidden inside `oops rx` to prevent recursive SSH loops.

### Workspace Cloning (`oops box clone <ssh-target> [-r <remote>] [path]`)
- **Top-Level Workspace Creation**: Clones an existing Oopsbox workspace from a remote server over SSH without requiring external Git hosts.
- **Execution Workflow**:
  1. Clones `~/.oops/oopsbox.git` (or `~/oopsbox` fallback) from `<ssh-target>` into local `[path]`.
  2. Automatically registers local Git remote `oops-<remote>` (default: `prod`) pointing to `<ssh-target>:.oops/oopsbox.git`.
  3. Outputs clear post-clone guidance for local editing and `oops deploy`.

## Interface & Subcommands Specification

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| `oops box clone` | `<ssh-target> [-r <remote>] [path]` | Clones workspace from remote server over SSH and auto-pairs local Git remote |
| `oops remote add` | `<ssh-target> [-r <name>]` | Registers remote server via SSH (namespaced as `oops-<name>` in Git), sets up local Git remote |
| `oops remote list` | *(none)* | Lists registered remote servers (displays logical names without `oops-` prefix) |
| `oops remote remove` | `<name>` | Removes registered remote server configuration (`oops-<name>`) |
| `oops remote rename` | `<old-name> <new-name>` | Renames registered remote configuration from `oops-<old-name>` to `oops-<new-name>` |
| `oops deploy` | `[-r <remote>] [ref]` | Top-level command for deploying workspace updates to remote server using `-r <remote>` (default: `prod`) |
| `oops rx` | `[-r <remote>] <cmd> [args...]` | Remote Execute: Forwards any `oops` command over SSH to remote server `~/oopsbox` using `-r <remote>` (default: `prod`) |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops rx [-r <remote>] <cmd>`, `oops deploy [-r <remote>] [ref]`).
- **Downstream Dependencies**: Native SSH client (`ssh`), Git CLI (`git`), Docker Engine & `docker-compose-plugin`.
- **Bounded Blast Radius**:
  - `cmd/remote.go`: Subcommand routing for `oops remote`.
  - `internal/remote/`: Remote server registration, SSH command execution delegator, bare sync runner.
  - `docs/specs/oopsbox-bare-sync.md`: Living specification.

## Verification & Acceptance Criteria

- **Unit Tests**:
  - `go test -v -race ./internal/remote/... ./cmd/...` exits with Code 0.
  - Remote command forwarding constructs correct SSH invocation strings.
  - Post-receive hook generator outputs correct script permissions (`0755`).
- **CLI Verification**:
  - `oops remote --help` displays all subcommands cleanly.
  - `oops remote list` displays configured remotes.
