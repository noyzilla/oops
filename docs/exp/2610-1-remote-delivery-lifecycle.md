---
id: "exp-2610-1"
title: "Remote Delivery Lifecycle & Multi-Stack Capsule Separation"
status: active
tags: [remote, deploy, sync, capsule, multi-stack, lifecycle]
crystallization:
  target: spec
synapses: ["docs/specs/remote.md", "docs/specs/cli.md", "docs/specs/env-sync.md"]
---

# exp-2610-1: Remote Delivery Lifecycle & Multi-Stack Capsule Separation [สถาปัตยกรรมส่งมอบข้อมูลและการแยกแคปซูลหลายสแตก]

## Problem & Context [ปัญหาและบริบทแวดล้อม]

During the evolution of the Oops remote workflow, three architectural frictions emerged:

- **Command Overlap between `deploy` and `sync push` [ความซ้ำซ้อนระหว่าง deploy และ sync push]**:
  - `oops deploy` currently streams Base manifests (`compose.yaml`, `services/`, `config/`), streams the Capsule overlay (`/@<remote>/`), and then triggers `docker compose up -d`.
  - `oops sync push` uploads `.env` and `config/secrets/` from `/@<remote>/` to the remote server.
  - This causes conceptual redundancy: operators question whether editing a configuration in `/@<remote>/` requires `oops sync push` or `oops deploy`.
- **Multi-Stack File Scoping (`compose.<stack>.yaml`) [ความเสี่ยงการชนกันเมื่อมีหลายสแตกใน Workspace เดียวกัน]**:
  - In Oopsbox, application stacks are flat files at the workspace root: `compose.<stack>.yaml` (e.g. `compose.crm.yaml`, `compose.store.yaml`).
  - Deploying to a remote server without scoping risks leaking unrelated stack manifests (`compose.store.yaml`) onto a production server intended only for `/crm`.
  - Deploying must cleanly isolate target stack manifests and their specific dependencies without sending inactive sibling stack files.
- **Implicit vs Explicit Container Execution [ความเสี่ยงจากการรัน Container โดยอัตโนมัติ]**:
  - Coupling manifest delivery with container restart inside `oops deploy` creates operational risks (e.g. unexpected downtime during pre-staging).
  - Manifest delivery should be decoupled from runtime orchestration.

## Motivation & Objectives [เป้าหมายและผลลัพธ์ที่ต้องการ]

- **Strict Responsibility Separation**:
  - **Delivery (Data Transfer)**: `oops deploy` transfers manifests and overlays with zero side-effects.
  - **Lifecycle Execution**: `oops rx` controls remote container operations (`up`, `down`, `restart`, `ps`).
  - **State Synchronization**: `oops remote pull` and `oops remote push` synchronize runtime configurations and secrets with local capsules.
- **Hermetic Capsule Isolation**:
  - The local root workspace (`.env`, `compose.local.yaml`, `compose.<stack>.local.yaml`) remains permanently untouched during remote synchronization.
  - Remote configurations reside exclusively within `/@<remote>/`.
- **Scoped Multi-Stack Filtering**:
  - Operators can scope deployment and synchronization to a specific stack (`/<stack>`), preventing cross-stack pollution.

## Hypotheses & Evaluated Options [ทางเลือกและสมมติฐานที่ประเมิน]

- `[!]` **Option 1: Retain Status Quo with Clarified Documentation [คงคำสั่งเดิมแต่ขยายเอกสาร]**:
  - Keep `oops deploy` with container restart and `oops sync push/pull` as separate commands.
  - *Rejected*: Fails to resolve the underlying command overlap and leaves container execution coupled to manifest transfer.
- `[*]` **Option 2: Unified Symmetric Remote Model (Git Parity) [แนวทางความสมมาตรแบบ Git: Push / Pull / Rx]**:
  - Unify all outgoing uploads under **`oops remote push [/<stack>] [@<remote>]`** (streams base manifests, referenced templates, and capsule overlays).
  - Retain **`oops deploy`** as a direct ergonomic alias for `oops remote push` to preserve DevOps muscle memory.
  - Pair symmetrically with **`oops remote pull [/<stack>] [@<remote>]`** (downloads remote configs into capsule via `.oopsallow`).
  - Strip container execution (`docker compose up -d`) from outgoing delivery, making `push` / `deploy` pure manifest and overlay streaming.
  - Delegate container execution entirely to **`oops rx [@<remote>] <oops-cmd> [args...]`** (e.g. `oops rx @prod up`).
  - Support stack scoping `[/<stack>]` across `push`, `pull`, and `rx`.
  - **Drop `oops sync` entirely [ยกเลิกคำสั่ง oops sync โดยสมบูรณ์]**: Eliminate alias technical debt and enforce the unified push/pull/rx mental model.
  - *Chosen Candidate*: Provides clean separation of concerns, zero confusion between push and deploy, seamless multi-stack isolation, and perfect Git symmetry.
- `[!]` **Option 3: All-in-One Deploy with Flag Gating [รวมทุกอย่างไว้ใน deploy ด้วย Flag]**:
  - Eliminate `push` entirely and rely on `oops deploy --config-only`.
  - Use `oops pull` exclusively for downloads.
  - *Rejected*: Overloads `deploy` with dual semantics (code delivery vs configuration patching) and creates cognitive friction.

## Architectural Model [แบบจำลองทางสถาปัตยกรรม]

### Directory Mirroring Topology [โครงสร้างการสะท้อนโฟลเดอร์]

Application stacks in Oopsbox reside as flat root files `compose.<stack>.yaml`, optionally paired with machine overrides `compose.<stack>.local.yaml`.
Root `./compose.yaml` is strictly local to the workstation and is NEVER sent to remote servers. Each remote environment maintains its own base manifest inside its capsule:

```text
Local Workspace (Machine A)                    Remote Server (~/oopsbox)
├── compose.yaml [Local Dev Only]              ├── compose.yaml (มาจาก /@prod/compose.yaml)
├── oops.yaml [Local Dev Only]                 ├── oops.yaml (มาจาก /@prod/oops.yaml)
├── compose.crm.yaml                           ├── compose.crm.yaml
├── compose.store.yaml [Local Dev Only]        ├── services/<เฉพาะที่ใช้>
├── compose.local.yaml [Local Dev Only]        ├── config/<เฉพาะที่ใช้>
├── compose.crm.local.yaml [Local Dev Only]    ├── compose.local.yaml [Remote Prod]
├── services/                                  ├── compose.crm.local.yaml [Remote Prod]
├── config/                                    └── .env [Remote Prod Root]
│                                                             ▲
└── /@prod/ [Capsule Overlay]                                 │
    ├── remote.yaml                                           │
    ├── .oopsallow [Sync Whitelist - จาก .oopsallow.example] ─┤ (กฎกรองไฟล์ push/pull)
    ├── compose.yaml [Remote Base - จาก compose.yaml.example] ┤ (ส่งเป็น ~/oopsbox/compose.yaml)
    ├── oops.yaml [Remote Engine - จาก oops.yaml.example] ────┤ (ส่งเป็น ~/oopsbox/oops.yaml)
    ├── .env [Remote Prod Root] ──────────────────────────────┤ (via remote push/pull)
    ├── compose.local.yaml [Remote Server Override] ──────────┤
    ├── compose.crm.local.yaml [Remote CRM Override] ─────────┤
    └── config/ (คัดลอกไปทั้งโฟลเดอร์ 1:1) ───────────────────┤ (ส่งเป็น ~/oopsbox/config/)
        ├── env/*                                             │
        ├── secrets/*                                         │
        └── <service>/*                                       ┘
```

### Data Transfer & Selective Copying Model [โมเดลการคัดลอกและเลือกส่งข้อมูล]

When deploying from local `~/oopsbox` to remote target `@remote:~/oopsbox`, data transfer follows a 2-step selective copy:

```text
Local Machine: ~/oopsbox                               Remote Server: @remote:~/oopsbox
========================                               ==============================

[Step 1: Base Manifests (Selective & Capsule Sovereign)]
├── /@remote/compose.yaml (Capsule Base) ─────────────> ~/oopsbox/compose.yaml
├── /@remote/oops.yaml (Capsule Orchestrator) ────────> ~/oopsbox/oops.yaml
├── compose.<stack>.yaml (เฉพาะสแตกที่เลือก) ──────────> ~/oopsbox/compose.<stack>.yaml
├── services/<เฉพาะที่ใช้> ───────────────────────────> ~/oopsbox/services/
└── config/<เฉพาะที่ใช้> ─────────────────────────────> ~/oopsbox/config/
    (คัดลอกเฉพาะสแตกที่เลือก ห้ามส่ง compose.<other>.yaml ไปปน)
    (./compose.yaml, ./oops.yaml และ oopsbox.yaml ของ local ไม่ส่งขึ้น server)

[Step 2: Capsule Overlay]
└── /@remote/* ──────────────────────────────────────> ~/oopsbox/
    ├── .env (prod root) ────────────────────────────> ~/oopsbox/.env
    ├── compose.local.yaml (prod root override) ─────> ~/oopsbox/compose.local.yaml
    ├── compose.<stack>.local.yaml (prod override) ──> ~/oopsbox/compose.<stack>.local.yaml
    └── config/* (คัดลอกไปทั้งหมด 1:1) ───────────────> ~/oopsbox/config/
    (ยกเว้น remote.yaml, compose.yaml และ oops.yaml ที่ส่งใน Step 1 แล้ว)
```

#### Capsule-First Manifests & Blueprint Init [การแยกฐานรากแคปซูลและการสร้างค่าเริ่มต้นจาก Blueprint]
- **Zero Local Manifest Leakage [ปลอดการรั่วไหลจาก Local 100%]**:
  - Root `./compose.yaml` และ `./oops.yaml` เป็นของเครื่อง Local Dev เท่านั้น **ห้ามคัดลอกหรือส่งไปทับบน Remote Server เด็ดขาด**
  - ทุก Remote Server จะใช้ `/@<remote>/compose.yaml` และ `/@<remote>/oops.yaml` ประจำแคปซูลของตัวเองเป็น Base และ Orchestrator Manifest เสมอ
- **Blueprint-Driven Initialization [การสร้างค่าเริ่มต้นเมื่อเชื่อมต่อเครื่องใหม่]**:
  - เมื่อสั่ง `oops remote connect @remote`:
    - หากเซิร์ฟเวอร์ปลายทางมี `compose.yaml`, `oops.yaml`, หรือ `.oopsallow` อยู่แล้ว: ระบบจะ Auto Pull ลงมาเก็บที่ `/@<remote>/`
    - หากเป็นเซิร์ฟเวอร์ใหม่ที่ยังไม่มีไฟล์ดังกล่าว: ระบบจะคัดลอกสร้างจากไฟล์ Blueprint ตัวอย่างใน Root Project:
      - `/@<remote>/compose.yaml` สร้างจาก **`compose.yaml.example`** (โครงข่าย Network กลางแบบสะอาด)
      - `/@<remote>/oops.yaml` สร้างจาก **`oops.yaml.example`** (Orchestrator Compose Manifest สำหรับตัว Oops Engine)
      - `/@<remote>/.oopsallow` สร้างจาก **`.oopsallow.example`** (Whitelist กำหนดขอบเขตไฟล์ที่อนุญาตให้ส่ง-รับ)
  - **Single Source of Truth Invariant**: ทั้งการ `push` (ส่งขึ้น) และ `pull` (ดึงลง) ในครั้งต่อไป **จะต้องยึดถือตาม `.oopsallow` เท่านั้น** ป้องกันการปนเปื้อนข้อมูลและการหลุดส่งไฟล์ที่ไม่ได้รับอนุญาต 100% (ยกเว้น `compose.<stack>.yaml` และ `services/` ที่ถูกดึงจาก Root ตาม Dependency Graph)

#### Selective Packaging Invariants [กฎเกณฑ์การเลือกคัดลอกข้อมูล]
- **Services Dependency Resolution Algorithm [ขั้นตอนการวิเคราะห์ Service ที่ถูกใช้งานจริง]**:
  To prevent copying unused templates from `services/*.yaml`, the packager resolves the union set of required services through two inspection sources:
  1. **Source 1 — Stack & Remote Base Inspection (`compose.<stack>.yaml` and `/@<remote>/compose.yaml`)**:
     - **Tag Inspection (`x-oops-services:`)**: Scans root-level declarative service dependencies (e.g. `x-oops-services: [postgres, redis, caddy]`).
     - **Container Inspection (`depends_on:`)**: Scans all `depends_on:` declarations across defined container services.
  2. **Source 2 — Remote Capsule Overlay Inspection (`/@<remote>/`)**:
     - When targeting a remote host (`@remote`), additionally inspects `/@<remote>/compose.local.yaml` and `/@<remote>/compose.<stack>.local.yaml` for remote-specific service additions or override dependencies.
  3. **Packaging Result**:
     - Streams strictly the union set of discovered service templates: `services/<service>.yaml`.
     - Streams matching service configurations: `config/<service>/`.
- **Stacks Selective Copy (`compose.<stack>.yaml`)**:
  - When deploying `oops deploy /crm @remote`: Streams strictly `compose.crm.yaml` and excludes all other sibling stack files (e.g. `compose.store.yaml`).
  - When deploying unscoped `oops deploy @remote`: Streams all declared stack compose files.
- **Capsule Overlay Copy (`~/oopsbox/@remote/*`)**:
  - Streams overlay configs from local `/@remote/` directly to root `~/oopsbox/` on the remote server.
  - **Full Capsule Config Stream**: คัดลอกโฟลเดอร์ `/@<remote>/config/` ไปทั้งโฟลเดอร์ (`/@<remote>/config/*` -> `~/oopsbox/config/`) ครอบคลุมทั้ง `config/env/`, `config/secrets/`, และ custom configs ของ remote นั้นๆ
  - Automatically excludes `remote.yaml` (connection metadata stays local).
- **Global Exclusions**:
  - Never copy: `./compose.yaml` และ `./oops.yaml` from local root, `*.local.yaml` from local root (local dev machine overrides), `/@*` (other capsules), `.git/`, `.scratch/`, `data/`, and all local `.env` files.

### Command Responsibility Matrix [ตารางแบ่งขอบเขตหน้าที่ของคำสั่ง]

| Command | Target Scope | Action Performed | Container Impact |
| :--- | :--- | :--- | :--- |
| `oops remote push` *(or `oops deploy`)* | `[/<stack>] [@<remote>]` | Streams base manifests, referenced templates, and capsule overlay via SSH | **No restart** (Pure delivery) |
| `oops remote pull` | `[/<stack>] [@<remote>]` | Downloads remote configs and secrets into `/@<remote>/` via `.oopsallow` | No restart |
| `oops rx` | `[@<remote>] <oops-cmd> [args...]` | Forwards Oops CLI commands (e.g. `up`, `ps`, `restart`, `logs`) | **Operator controlled** |

### Delivery & Sync Workflows [โฟลว์การทำงานของคำสั่ง]

#### 1. Outgoing Delivery: `oops remote push` *(Alias: `oops deploy`)*
Uploading manifests and configurations to remote server:
- **Step 1 — Base Manifest Streaming**:
  - Scoped (`/<stack>` specified, e.g. `/crm`): Streams shared core (`/@<remote>/compose.yaml`, `/@<remote>/oops.yaml`, `services/<เฉพาะที่ใช้>.yaml`, `config/<เฉพาะที่ใช้>`) and specifically `compose.crm.yaml`. Excludes all other `compose.*.yaml` files.
  - Unscoped (no `/<stack>`): Streams the entire workspace tree (`/@<remote>/compose.yaml`, `/@<remote>/oops.yaml`, all `compose.*.yaml`, referenced `services/`, `config/`).
  - Always Excludes: `*.local.yaml`, `/@*`, `.git`, `.scratch`, `data/`, and all `.env` files of local development.
- **Step 2 — Overlay Application (Filtered via `.oopsallow`)**:
  - Streams files in `/@<remote>/` **strictly matching `/@<remote>/.oopsallow`** (e.g. `compose.yaml`, `oops.yaml`, `.env`, `compose.local.yaml`, `compose.<stack>.local.yaml`, และสตรีมไฟล์ทั้งหมดใน `config/*`) ไปวางทับบน Server แบบ 1:1
  - Any file in `/@<remote>/` not permitted by `.oopsallow` is blocked from transmission.
- **Post-Delivery Hint**: Displays summary of transferred manifests and prompts operator:
  `==> Push complete. Run 'oops rx [@<remote>] up [/<stack>]' to start or restart services.`

#### 2. Incoming Synchronization: `oops remote pull`
Downloading configurations and secrets from remote server into local capsule:
- **The `.oopsallow` Whitelist Protocol**:
  - Connects to remote server over SSH.
  - **Fail-Closed Security (Default-Deny)**: To protect local workstations from downloading massive databases (`data/`), backup archives (`backups/`), or log files, the pull engine filters server files strictly using **`.oopsallow`**:
    - **Configuration File**: Reads patterns from `/@<remote>/.oopsallow` (capsule-specific), falling back to root `./.oopsallow` or the built-in system defaults:
      ```text
      .env
      compose.yaml
      compose*.local.yaml
      config/**
      oops.yaml
      ```
    - **Scoped Filtering**: When `/<stack>` is specified, restricts `.env` and `compose.<stack>.local.yaml` to that stack, alongside shared configs.
    - **Matching Downloads**: Downloads exclusively files that match the `.oopsallow` patterns into `/@<remote>/` using the 1:1 mirror structure.
    - **Hard-Deny Invariant**: Anything not matched in `.oopsallow` (especially `data/`, `backups/`, `.git/`, `*.log`, `*.sock`) is completely ignored and never downloaded.
  - **Zero Local Mutation**: Local root files (`.env`, `compose.*.local.yaml`) are strictly untouched.

### Multi-Stack Scoping Invariants [กฎเกณฑ์ความปลอดภัยสำหรับหลายสแตก]

- **Stack Scoped Deploy (`oops deploy /crm @prod`)**:
  - Base Stream includes: `compose.yaml`, `compose.crm.yaml`, referenced `services/*.yaml`, and referenced `config/`.
  - Base Stream strictly excludes: `compose.store.yaml` (and all other stacks), `*.local.yaml`, `/@*`, `.git`, `.scratch`, `data/`, and all local `.env` files.
  - Overlay Stream includes: `/@prod/.env`, `/@prod/compose.crm.local.yaml`, and `/@prod/config/`.
  - Remote state: Only `/crm` files are updated; other existing stack compositions on the server remain untouched.
- **Capsule Preservation (Zero Local Mutation)**:
  - Invoking `oops remote pull` writes exclusively into `/@<remote>/`.
  - Local root `.env` and `compose.crm.local.yaml` are never read during `push` and never overwritten during `pull`.

## Discussion & Trade-offs [บันทึกการอภิปรายและการประเมิน]

### 2026-10-11
- Evaluated overlap between `oops deploy` and `oops sync push`:
  - Verified that Step 2 of `deploy` was duplicating `sync push`.
  - Identified operator confusion regarding which command to run when updating environment variables.
- Evaluated container restart coupling:
  - Confirmed that automatic `docker compose up -d` in `oops deploy` prevents staging changes ahead of maintenance windows.
  - Agreed that separating manifest streaming from container execution aligns with the core philosophy of `oops rx`.
- Evaluated dropping legacy `oops sync`:
  - Decided to eliminate `oops sync` completely instead of keeping it as a backward-compatible alias.
  - Dropping the command eliminates maintenance overhead, prevents confusion between duplicate interfaces, and accelerates operator adoption of the canonical `oops remote pull/push` model.
- Aligned project stacks with Oopsbox ubiquitous language:
  - Standardized terminology to **Stack** (`/<stack>`) representing application compositions `compose.<stack>.yaml` and overrides `compose.<stack>.local.yaml`.
  - Designed selective service packaging (`services/<เฉพาะที่ใช้>`) based on active compose dependency tree.

### 2026-10-12
- Decoupled orchestrator manifest into Compose format:
  - Formatted `oops.yaml` as a valid Docker Compose manifest managing the `oops` daemon container, embedding system settings under `x-oops:` extension fields.
  - Kept `oopsbox.yaml` strictly for developer workstation VM engines (Colima / OrbStack) without uploading to servers.
- Evaluated Remote Base Manifest Sovereignty (`compose.yaml`):
  - Rejected inheriting local `./compose.yaml` to prevent local development configurations (debugging ports, bind mounts) from leaking into remote environments.
  - Adopted **Capsule-First Base with Minimal Skeleton Init (Option 2)**:
    - Root `./compose.yaml` remains strictly local to the workstation.
    - Each remote environment maintains its own `/@<remote>/compose.yaml`.
    - Fresh servers initialize `/@<remote>/compose.yaml` with a clean minimal network skeleton (`name: oops`, `networks: default: name: oops-network`).

### 2026-10-13
- Standardized Full Capsule Config Streaming:
  - Confirmed that Step 2 (Capsule Overlay) streams all files in `/@<remote>/config/*` in their entirety (covering all envs, secrets, and service overrides) directly to `~/oopsbox/config/`.
- Evaluated Pull Safety Protocol (`.oopsallow` vs `.oopsignore`):
  - Selected **`.oopsallow` (Default-Deny Whitelist)** over `.oopsignore` (Default-Allow Blacklist).
  - Guarantees fail-closed safety by only pulling known configuration patterns (`.env`, `compose.yaml`, `compose*.local.yaml`, `config/**`, `oops.yaml`), permanently preventing accidental multi-gigabyte downloads of database data (`data/`), dumps (`backups/`), or runtime logs.
- Unified `oops deploy` and `oops remote push` (Symmetric Git/Remote Model):
  - Resolved command redundancy between `deploy` and `push` by merging them into a single symmetric workflow:
    - Outgoing: `oops remote push` (with `oops deploy` as an exact ergonomic alias).
    - Incoming: `oops remote pull`.
    - Execution: `oops rx`.
  - Eliminates operator confusion over which command to use when updating remote environments.

## Open Questions [คำถามที่รอการพิสูจน์]

- [ ] Should `/@<remote>/remote.yaml` optionally support an `active_stacks` whitelist to prevent accidental deployment of unapproved stacks to production?
- [ ] Should `oops deploy` print a reminder hint recommending `oops rx [@<remote>] up` upon completion of file streaming?

## Next Steps [ขั้นตอนการนำไปปฏิบัติ]

- [ ] Update Living Specifications:
  - Reflect pure streaming and selective service copy in [docs/specs/remote.md](../specs/remote.md).
  - Update CLI reference in [docs/specs/cli.md](../specs/cli.md) with `oops remote pull/push` and remove `oops sync`.
- [ ] Refactor Codebase:
  - Remove `docker compose up -d` from [cmd/deploy.go](../../cmd/deploy.go).
  - Implement selective stack (`compose.<stack>.yaml`) and service (`services/<name>.yaml`) filtering in `cmd/deploy.go`.
  - Add `pull` and `push` subcommands to [cmd/remote.go](../../cmd/remote.go).
  - Remove [cmd/sync.go](../../cmd/sync.go) and unregister `sync` from [cmd/root.go](../../cmd/root.go).
  - Extend [internal/sync/sync.go](../../internal/sync/sync.go) to support selective stack overrides.
