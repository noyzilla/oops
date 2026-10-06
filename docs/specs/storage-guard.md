---
title: Storage Guard
status: active
tags: [storage, mount, symlink, priority, safety, oopsbox]
synapses: ["CONTEXT.md", "docs/specs/oopsbox.md", "docs/specs/cli.md"]
---

# Specification: Storage Guard [ตัวป้องกันข้อมูลหลุดลง OS disk]

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Companion Specs**: [oopsbox.md](oopsbox.md), [cli.md](cli.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)

## Overview & Scope

Storage Guard validates that persistent storage links of an oopsbox are healthy before any container is started or recreated. It prevents a stack from silently writing data to the OS disk when a persistent disk is unmounted or a symlink is dead. [ตรวจ data/ และ backups/ ก่อนสั่ง container ทำงาน]

Scope:
- Checked paths: `<oopsbox>/data` and `<oopsbox>/backups`, and only when they are symlinks. A real directory always passes.
- Guarded commands: `oops up`, `oops restart`, `oops update`, `oops switch`, and `oops box start`. Webhook deploy (Docker API recreation of existing containers) is out of scope and not guarded.

## Domain Context & Ubiquitous Language

- **Storage Link**: A symlink named `data` or `backups` at the oopsbox root, pointing to a persistent disk path.
- **Mount Prefix**: A directory prefix under which persistent disks are expected to be mounted. Matching respects directory boundaries and covers all depths.
- **Bad Link**: A Storage Link that is dead or whose target sits on the root filesystem under a Mount Prefix.
- **Priority List**: Ordered `priority` entries in `oops.yml`. Earlier entries have higher priority; unlisted targets have the lowest priority.
- **Blocked Service**: A service the guard refuses to start in the current operation.

## Business Rules & Logic Invariants

### Link Health
- A Storage Link is a Bad Link when its target does not exist (dead link).
- A Storage Link is a Bad Link when its resolved target path falls under a Mount Prefix and the device id of the target equals the device id of `/` (the target is on the root filesystem, so the disk is not mounted).
- Targets outside every Mount Prefix are not checked beyond dead-link detection.
- Prefix `/mnt` matches `/mnt/disks/x/y` but not `/mntx`. Prefix `/` matches every target.
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

### Blocking Behavior
- Affected services are those whose resolved bind-mount sources (from `docker compose config --env-file <oopsbox>/.env`) pass through a Bad Link.
- Every affected service is blocked. Every service with strictly lower priority than the highest-priority affected service is also blocked in the same operation.
- Same-priority services that do not use the Bad Link are not blocked.
- The `edge` stack is never blocked and is always started first, because the webhook server and DNS live in edge. This holds even when `priority` is unset.
- With no Bad Link, no service is blocked and behavior is unchanged.

## Flow & State Machine

- **Initial State**: Command resolved its target services.
- **Guard Evaluation**: Check Storage Links, find affected services, compute Blocked Services.
- **Execution**: Start unaffected services (edge first), skip Blocked Services, print the reason per Blocked Service.
- **Failure Recovery**: Exit non-zero when any service was blocked.

## Interface & Data Contracts

- **Config Contract**: `OopsConfig.Storage.MountPrefixes []string`, `OopsConfig.Priority []string`, loaded by `LoadOopsConfig`.
- **Package**: `internal/storage` provides link inspection, prefix matching, and root-device comparison. Filesystem and device lookups are injectable for tests.
- **Error Contract**: Message names the Storage Link, its target, the reason (dead or on OS disk), and the list of Blocked Services.

## Dependency & Blast-Radius Matrix

- **Upstream Callers (Inbound)**: `cmd/up.go`, `cmd/restart.go`, `cmd/update.go`, `cmd/switch.go`, `cmd/box.go`.
- **Downstream Dependencies (Outbound)**: `docker compose config`, `internal/docker` (resolver, config), operating system `stat`.
- **Bounded Blast Radius**: `internal/storage/`, `internal/docker/config.go`, `internal/orchestrator/updater.go`, `cmd/`.

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./internal/storage/... ./internal/docker/... ./internal/orchestrator/... ./cmd/...`
- **Happy Path**: Healthy links or real directories start all services with no output change.
- **Error Handling**: A dead link or an unmounted target under a Mount Prefix blocks affected and lower-priority services, starts the rest, and exits non-zero.
- **Boundary Cases**: `/mnt` vs `/mntx`, prefix `/`, empty prefix list, macOS dead-link-only mode.
