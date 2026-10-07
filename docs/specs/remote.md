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

This specification defines the `oops remote` command suite for remote server bootstrapping, Git Bare synchronization, and SSH-delegated remote orchestration. It strictly separates local workstation management (`oops box`) from remote server operations (`oops remote <server> <command>`), allowing developers to sync, deploy, and inspect remote environments (e.g. production, staging) via Native SSH without third-party Git hosts.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Local Workstation (`oops box`)**: Local developer workstation environment (OrbStack / Colima / Docker Desktop).
- **Remote Host (`oops remote`)**: Remote server target registered via SSH alias in `~/.ssh/config` or `user@host`.
- **Git Bare Repository (`<bare-path>`)**: Fixed server-side bare Git repository at `~/.oops/oopsbox.git` serving as central sync point.
- **Remote Workspace Path (`<oopsbox-path>`)**: Fixed working tree directory at `~/oopsbox` on the server.
- **Explicit Deploy Execution**: Deployments are driven explicitly via `oops remote deploy` or shorthand `oops deploy`, pushing commit history and running SSH-delegated checkout and container up.

## Business Rules & Logic Invariants

### Workspace Requirement Invariant
- `oops remote add`, `oops remote deploy`, and shorthand `oops deploy` MUST be executed within a valid local oopsbox workspace directory (`.git` or `stacks/` or `compose.yaml` present).
- If executed outside a valid oopsbox workspace directory, the command halts cleanly with:
  `Error: Not inside a valid oopsbox workspace directory`.

### Remote Server Registration (`oops remote add [name] <ssh-target>`)
- **Argument Resolution**:
  - **1 Argument (`oops remote add <ssh-target>`)**: If `<ssh-target>` is an SSH Host alias in `~/.ssh/config` (no `@` or `/`), `<name>` defaults to the alias name (e.g., `anthole`). Otherwise, `<name>` defaults to `oopsbox`.
  - **2 Arguments (`oops remote add <name> <ssh-target>`)**: `<name>` is explicitly set (e.g., `staging`), and `<ssh-target>` is the SSH target.
- **Fixed Server Locations**:
  - Server Bare Repository is strictly fixed at `~/.oops/oopsbox.git`.
  - Server Workspace Directory is strictly fixed at `~/oopsbox`.
- **Git Remote Registration**: Registers local Git remote `<name>` using standard SCP-style SSH notation: `<ssh-target>:.oops/oopsbox.git` (e.g., `anthole:.oops/oopsbox.git` or `captain@anthole.local:.oops/oopsbox.git`).
- **Google COS & Read-Only OS Support**:
  - Detects read-only filesystems (Google Container-Optimized OS / COS).
  - On Google COS, installs `oops` CLI into `/var/lib/google/bin/oops` and `docker-compose` plugin into `/var/lib/google/docker-cli-plugins/docker-compose`.
  - Configures `"cliPluginsExtraDirs": ["/var/lib/google/docker-cli-plugins"]` in `${HOME}/.docker/config.json` and `/root/.docker/config.json`.
- **Server Initialization**: Runs `oops box init-bare ~/.oops/oopsbox.git ~/oopsbox` on the server via SSH.

### Server-Side Bare Repository Initialization (`oops box init-bare <bare-path> <oopsbox-path>`)
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

### Explicit Remote Deployment (`oops remote deploy [name] [ref]` & `oops deploy [name] [ref]`)
- **Argument Defaults**:
  - `[name]`: Target remote name (Default: `oopsbox` or single registered remote if only one exists).
  - `[ref]`: Target Git reference to deploy (Default: active Git branch, e.g., `main`).
- **Execution Workflow**:
  1. Executes `git push <name> <ref>` to sync Git commits to remote `~/.oops/oopsbox.git`.
  2. Executes SSH to remote target:
     `git --git-dir=$HOME/.oops/oopsbox.git --work-tree=$HOME/oopsbox checkout -f <ref> && (oops up -C $HOME/oopsbox || /var/lib/google/bin/oops up -C $HOME/oopsbox)`

## Interface & Subcommands Specification

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| `oops remote add` | `[name] <ssh-target>` | Registers remote server via SSH (defaults fixed path `~/.oops/oopsbox.git` and `~/oopsbox`), sets up local Git remote |
| `oops remote list` | *(none)* | Lists registered remote servers |
| `oops remote remove` | `<name>` | Removes registered remote server configuration |
| `oops remote deploy` | `[name] [ref]` | Pushes commits and executes SSH-delegated checkout + `oops up` on remote server |
| `oops deploy` | `[name] [ref]` | Top-level shorthand alias for `oops remote deploy` |
| `oops remote <server> <cmd>` | `<up|down|ps|logs>` | Delegates `oops` command execution directly to the remote server over SSH |

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: Developer terminal (`oops remote <server> <cmd>`).
- **Downstream Dependencies**: Native SSH client (`ssh`), Git CLI (`git`), Docker Engine & `docker-compose-plugin`, `internal/key` package.
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
