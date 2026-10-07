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
- **Remote Host (`oops remote <server>`)**: Remote server target registered via SSH alias in `~/.ssh/config` (or IP/target string).
- **Git Bare Repository (`<bare-path>`)**: Server-side bare Git repository (e.g., `~/.oops/repos/<box-slug>.git`) serving as central sync point.
- **Remote Workspace Path (`<oopsbox-path>`)**: Working tree directory on the server (e.g., `~/oopsbox` or `/opt/oopsbox`).
- **Post-Receive Hook (`hooks/post-receive`)**: Git server hook checking native `-o deploy` push option to execute `git checkout -f <target_ref>` and `oops up -C <oopsbox-path>`.
- **Remote Command Delegation**: Forwarding `oops` CLI commands (`up`, `down`, `ps`, `logs`) to the remote server over SSH.

## Business Rules & Logic Invariants

### Remote Server Registration & Bootstrap (`oops remote add <name> <ssh-target> [remote-path]`)
- **SSH Target Resolution**: Resolves host config from `~/.ssh/config` or direct target string (`user@host:port`).
- **Google COS & Read-Only OS Support**:
  - Detects read-only filesystems (Google Container-Optimized OS / COS).
  - On Google COS, installs `oops` CLI into `/var/lib/google/bin/oops` and `docker-compose` plugin into `/var/lib/google/docker-cli-plugins/docker-compose`.
  - Configures `"cliPluginsExtraDirs": ["/var/lib/google/docker-cli-plugins"]` in `${HOME}/.docker/config.json` and `/root/.docker/config.json`.
- **Remote Tooling Verification & Install**:
  - Checks if `oops` CLI exists on remote server. If missing, installs to writeable binary path (`/var/lib/google/bin/oops` on COS, `~/.local/bin/oops` or `/usr/local/bin/oops` on standard Linux).
  - Checks if `docker` and `docker compose` plugin are installed. On standard Linux distros, installs `docker-compose-plugin` via package manager or `get.docker.com`. On COS, downloads plugin binary to `/var/lib/google/docker-cli-plugins/docker-compose`.
- **Server Bare Repository Initialization**: Runs `oops box init-bare <bare-path> <oopsbox-path>` on the server via SSH.
- **Local Remote Registration**: Adds Git remote `<name>` pointing to `ssh://<ssh-target>/<bare-path>` in the local workspace.

### Server-Side Bare Repository Initialization (`oops box init-bare <bare-path> <oopsbox-path>`)
- **Directory Setup**: Creates `<bare-path>` as a bare Git repository (`git init --bare <bare-path>`).
- **Post-Receive Hook (Explicit `-o deploy` Gate)**: Generates executable `hooks/post-receive` inside `<bare-path>`:
  ```bash
  #!/bin/sh
  export GIT_WORK_TREE="<oopsbox-path>"
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

  # Exit without checking out if no deploy option is present
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
    oops up -C "<oopsbox-path>"
  fi
  ```
- **Existing Workspace Seeding**: If `<oopsbox-path>` already exists with files/stacks, `init-bare` commits existing workspace state and pushes to `<bare-path>` as origin main.

### Remote Synchronization & Deployment (`oops remote <server> push` / `oops remote <server> deploy`)
- **`oops remote <server> push [-b branch]`**: Pushes Git commits to `<server>` bare repo for backup/history sync without `-o deploy`. The working tree `<oopsbox-path>` and running containers remain untouched.
- **`oops remote <server> deploy [-b branch] [-t tag]`**: Pushes specified branch or Git tag with `-o deploy` (`git push -o deploy <server> <ref>`), triggering file checkout into `<oopsbox-path>` and running `oops up -C <oopsbox-path>` live.

### Remote Command Delegation (`oops remote <server> <command>`)
- Forwards execution of `up`, `down`, `ps`, `logs` to the remote server via SSH:
  ```bash
  ssh <ssh-target> "oops <command> -C <oopsbox-path>"
  ```

## Interface & Subcommands Specification

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| `oops remote add` | `<name> <ssh-target> [remote-path]` | Registers remote server via SSH, verifies/installs `oops` & `docker compose`, initializes server bare repo, and sets up local remote |
| `oops remote list` | *(none)* | Lists registered remote servers |
| `oops remote remove` | `<name>` | Removes registered remote server configuration |
| `oops remote <server> push` | `[-b <branch>]` | Syncs commits to remote bare repo without touching live working tree or containers (Sync Only) |
| `oops remote <server> deploy` | `[-b <branch>] [-t <tag>]` | Deploys specified branch or Git tag to remote server and triggers `oops up` container restarts |
| `oops remote <server> pull` | *(none)* | Pulls latest remote workspace configuration to local machine |
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
