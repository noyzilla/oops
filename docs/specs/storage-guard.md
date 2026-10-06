---
title: Storage Guard
status: active
tags: [storage, mount, symlink, priority, safety, oopsbox]
synapses: ["CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/cli.md", "docs/architecture/data-persistent-storage.md"]
---

# Specification: Storage Guard [ตัวป้องกันข้อมูลหลุดลง OS disk]

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Companion Specs**: [oopsbox.md](oopsbox.md), [cli.md](cli.md)
- **Architecture Context**: [Persistent Data and Storage Path Abstraction](../architecture/data-persistent-storage.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)

## Overview & Scope

Storage Guard validates host paths that hold persistent data. It prevents a service from silently writing data to the OS disk when a persistent disk is unmounted or a symlink is dead. [ตรวจ path ที่ mount เข้า container และ backup dir]

Scope:
- **Container guard**: every bind-mount source (resolved from `docker compose config`) of the services an operation will start. No path is special-cased; `data/` is covered because compose binds through it.
- **Backup guard**: the backup directory (`OOPS_BACKUP_DIR`, default `./backups`), checked only when a backup, prune, or restore command runs. Backups are host-local, usually not mounted into compose, and never affect container management.
- **Operator tools**: `oops storage check` and `oops storage link`.
- Guarded container commands: `oops up`, `oops restart`, `oops update`, `oops switch`, and `oops box start`. Webhook deploy (Docker API recreation of existing containers) is out of scope and not guarded.

## Domain Context & Ubiquitous Language

- **Storage Path**: A host path used as a bind-mount source or as the backup directory.
- **Mount Prefix**: A directory prefix under which persistent disks are expected to be mounted. Matching respects directory boundaries and covers all depths.
- **Bad Path**: A Storage Path with a dead symlink anywhere along it, or that resolves under a Mount Prefix while on the same device as `/`.
- **Priority List**: Ordered `priority` entries in `oops.yml`. Earlier entries have higher priority; unlisted targets have the lowest priority.
- **Blocked Service**: A service the guard refuses to start in the current operation.

## Business Rules & Logic Invariants

### Path Health
- A path is a Bad Path when any component along it is a symlink whose target does not exist (dead link).
- A path is a Bad Path when its resolved location falls under a Mount Prefix and its device id equals the device id of `/` (the disk is not mounted).
- Real directories and links that resolve outside every Mount Prefix pass, subject to the dead-link rule.
- A plain missing bind source (no dead link) is ignored for containers, because the container runtime creates it on first run.
- A missing backup directory is checked through its nearest existing ancestor, because creating it would land on that ancestor's disk.
- Prefix `/mnt` matches `/mnt/disks/x/y` but not `/mntx`. Prefix `/` matches every path.
- The mount check is effective on Linux only. On macOS (Docker runs in a VM) only dead-link detection applies.

### Configuration
```yaml
storage:
  mount_prefixes: [/mnt, /media, /Volumes]   # default when omitted
priority:
  - /edge
  - httpbin
  - /db
  - "@core"
  - /apps
```
- Setting `mount_prefixes` replaces the default list. An empty list disables the mount check but dead-link detection stays on.
- `priority` entries are `/stack`, `service`, or `@group`. The first matching entry decides the priority of a target.

### Backup Behavior
- `oops backup`, `backup-db`, `backup-data`, their `prune` subcommands, and `oops restore*` refuse to run when the backup directory is a Bad Path and exit non-zero. No container is blocked.
- `OOPS_BACKUP_DIR` may point anywhere on the host and need not be inside the oopsbox.

### Operator Tools
- `oops storage check`: inspects every bind source of every stack plus the backup directory, prints each Bad Path with its reason, exits non-zero when any is found.
- `oops storage link <target> [--name data] [--force]`: creates `<oopsbox>/data` as a symlink to `<target>` after verifying the target is an existing, healthy directory. An existing symlink is replaced only with `--force`. A directory that contains data is never replaced; an empty directory is.

### Blocking Behavior
- Affected services are those whose resolved bind-mount sources (from `docker compose config --env-file <oopsbox>/.env`) is a Bad Path.
- Every affected service is blocked. Every service with strictly lower priority than the highest-priority affected service is also blocked in the same operation.
- Same-priority services that use only healthy paths are not blocked.
- The `edge` stack is never blocked and is always started first, because the webhook server and DNS live in edge. This holds even when `priority` is unset.
- With no Bad Path, no service is blocked and behavior is unchanged.

## Flow & State Machine

- **Initial State**: Command resolved its target services.
- **Guard Evaluation**: Check every bind-mount source, find affected services, compute Blocked Services.
- **Execution**: Start unaffected services (edge first), skip Blocked Services, print the reason per Blocked Service.
- **Failure Recovery**: Exit non-zero when any service was blocked.

## Interface & Data Contracts

- **Config Contract**: `OopsConfig.Storage.MountPrefixes []string`, `OopsConfig.Priority []string`, loaded by `LoadOopsConfig`. CLI: `oops storage check`, `oops storage link <target> [--name] [--force]`.
- **Package**: `internal/storage` provides path inspection, prefix matching, root-device comparison, and link creation. Filesystem and device lookups are injectable for tests.
- **Error Contract**: Message names the Bad Path, its target, the reason (dead or on OS disk), and the list of Blocked Services.

## Dependency & Blast-Radius Matrix

- **Upstream Callers (Inbound)**: `cmd/up.go`, `cmd/restart.go`, `cmd/update.go`, `cmd/switch.go`, `cmd/box.go`, `cmd/backup.go`, `cmd/restore.go`, `cmd/storage.go`.
- **Downstream Dependencies (Outbound)**: `docker compose config`, `internal/docker` (resolver, config), operating system `stat`.
- **Bounded Blast Radius**: `internal/storage/`, `internal/docker/config.go`, `internal/orchestrator/guard.go`, `internal/orchestrator/updater.go`, `cmd/`.

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./internal/storage/... ./internal/docker/... ./internal/orchestrator/... ./cmd/...`
- **Happy Path**: Healthy paths start all services with no output change.
- **Error Handling**: A dead link or an unmounted path under a Mount Prefix blocks affected and lower-priority services, starts the rest, and exits non-zero.
- **Boundary Cases**: `/mnt` vs `/mntx`, prefix `/`, empty prefix list, macOS dead-link-only mode, missing backup directory, `storage link` replacement rules.
