---
id: "exp-2610-1"
title: "Compose Taxonomy, Discovery & Routing Protocol"
status: active
tags: [exploration, discovery, docker, compose, routing, overlay, env-sync, remote]
crystallization:
  target: spec
synapses: ["docs/specs/cli.md", "docs/specs/oopsbox.md", "docs/specs/env-sync.md", "docs/specs/remote.md"]
---

# exp-2610-1: Compose Taxonomy, Discovery & Routing Protocol [ระเบียบแบบแผนการจัดโครงสร้าง ค้นหา และกำหนดเส้นทาง Compose]

## Context [บริบทและความเป็นมา]
`oops` acts as an ergonomic orchestration layer over `docker compose`. The core design objectives are:
- Eliminate manual concatenation of `-f compose.yaml -f ...` arguments.
- Separate pure service templates from environment-specific overrides without coupling to Docker Compose's `include` directive.
- Establish clean directory-level environment overlays (`/@<remote>/`) that preserve 1:1 parity with remote servers under a dumb server model.
- Protect workstation resources by preventing production-only tooling (such as monitoring suites and log collectors) from running during local testing.
- Eliminate server-side Git dependencies (bare repositories, hooks, and Git remotes) in favor of Direct SSH streaming and 1:1 directory synchronization.

---

## Architectural Principles & Invariants [หลักการทางสถาปัตยกรรมและกฎเกณฑ์คงที่]

### Multi `-f` Chaining over `include` Directive [การต่อสาย `-f` แทนการใช้ `include`]
- **Root Working Directory Invariant**: Docker Compose resolves all relative bind mounts against the directory of the first specified `-f` file. By ensuring that root `compose.yaml` is always the first file in the chain, all module bind mounts (e.g., `./data/mysql`, `./config/mysql`) resolve directly against project root `/app` without requiring parent directory escapes (`../`).
- **No `include` Coupling**: Docker Compose `include` is discarded in favor of deterministic `-f` flag concatenation managed by `oops`.

### Ecosystem-Wide Strict `.yaml` Extension & Legacy Deprecation Invariant [ระเบียบการใช้นามสกุล `.yaml` ล้วนทั่วทั้งระบบ]
- **Universal `.yaml` Standard**: All configuration files and Compose manifests across the entire Oops ecosystem strictly use the canonical 4-letter `.yaml` extension (`compose.yaml`, `services/<service>.yaml`, `remote.yaml`, `oops.yaml`, `oopsbox.yaml`, `oopsbox.yaml.example`).
- **Legacy Naming Deprecation**: Support for `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `oops.yml`, `oopsbox.yml`, and all `*.yml` variants is permanently dropped. All discovery, validation, and execution engines strictly match `*.yaml` files only.

### Template Purity Invariant [โฟลเดอร์แม่แบบบริสุทธิ์]
- `services/` contains only pure, standardized service templates (e.g., `services/mysql.yaml`, `services/redis.yaml`).
- Machine overrides and environment overrides are strictly forbidden inside `services/`.

### Tail-Chaining Override Invariant [การจัดลำดับ Override ไว้ต่อท้ายสุดเสมอ]
- In Docker Compose, rightmost configuration files possess highest evaluation precedence.
- All untracked local override files (`*.local.yaml`) are strictly appended to the tail of the `-f` flag pipeline, ensuring that machine-specific developer settings (e.g., custom port mappings, local volume paths) reliably override all upstream base and project configurations.

### Root-Level Environment Overlay Model (`/@<remote>/`) [โมเดลโฟลเดอร์ Overlay ที่ Root]
- All remote environment specializations are encapsulated under top-level directories named `/@<remote>/` (e.g., `/@prod/`, `/@staging/`).
- Eliminates context fragmentation caused by scattered suffix naming (`.prod.env`, `my.prod.cnf`, `compose.prod.yaml`).
- During deployment, the contents of `/@<remote>/` are overlaid directly onto the remote target root.

### Zero-Git Server & Direct SSH Protocol [การยกเลิก Git บน Server และการใช้งาน Direct SSH]
- **No Server Git Dependency**: Remote deployment and synchronization eliminate all server-side Git requirements (no `git init --bare`, no `post-receive` hooks, no Git remotes). The target host requires only `ssh` and `docker`.
- **Self-Contained Remote Capsule**: Connection parameters are defined locally inside `/@<remote>/remote.yaml` referencing SSH host aliases from `~/.ssh/config`.
- **Direct SSH Streaming**: Deployments stream manifests directly over SSH (tar stream), unpacking them cleanly at the target directory.

### Git Hygiene & 1:1 Direct Sync Protocol [สุขอนามัยของ Git และการซิงค์แบบ 1:1]
- **Zero Secrets in Git**: All `.env` files (including root `.env`, `compose.local.yaml`, and `/@<remote>/.env`) are strictly Git-ignored.
- **1:1 Mirror Sync**: `oops sync` transfers files directly between `/@<remote>/` and the remote root without filename suffix mangling or slicing (`stripRemoteSuffix` is deprecated). Developers can pull remote `.env` to `/@<remote>/.env` for editing and push it back safely.

### Shared Infrastructure Boundary [ขอบเขตของโครงสร้างพื้นฐานส่วนกลาง]
- Shared infrastructure services (such as shared databases and reverse proxies) represent machine-level utilities.
- Individual projects consume shared infrastructure via network addresses and credentials, and are strictly prohibited from mutating shared container definitions.

---

## Taxonomy & File Organization [การจัดหมวดหมู่และโครงสร้างไฟล์]

```text
project-root/
├── compose.yaml                    # Base stack and x-groups metadata
├── compose.local.yaml              # Global machine-specific untracked overrides (Git-ignored)
├── compose.<project>.yaml          # Project-level composition (e.g., compose.myapp.yaml)
├── compose.<project>.local.yaml    # Project-level machine untracked overrides (e.g., compose.myapp.local.yaml)
│
├── services/                       # Pure service templates (Templates only)
│   ├── mysql.yaml
│   ├── redis.yaml
│   └── grafana.yaml
│
├── config/                         # Base service configurations
│   ├── mysql/
│   │   └── my.cnf
│   └── caddy/
│       └── Caddyfile
│
├── data/                           # Persistent host volume directories
│   ├── mysql/
│   └── redis/
│
└── @prod/                          # Remote Environment Overlay Capsule for 'prod'
    ├── remote.yaml                 # SSH target connection (host, path; excluded from server sync)
    ├── compose.yaml                # Production-specific base overrides & additions
    ├── compose.<project>.yaml      # Project-specific production overrides (optional)
    ├── .env                        # Production environment variables (Git-ignored, syncable)
    └── config/                     # Production configuration overlays
        └── mysql/
            └── my.cnf              # Server-tuned database configuration
```

---

## Remote Target Management (`oops remote`) [การจัดการเป้าหมายเซิร์ฟเวอร์]

Each remote environment is self-contained within its `/@<remote>/` directory:

### Remote Configuration Schema (`/@<remote>/remote.yaml`)
```yaml
host: oops-prod       # SSH host alias declared in ~/.ssh/config
path: ~/oopsbox       # Working directory on target server (default: ~/oopsbox)
```

### CLI Command Operations
- **Add Remote**: `oops remote add <ssh-alias> -r <name>` creates `/@<name>/remote.yaml`.
- **List Remotes**: `oops remote list` discovers and prints all remotes by scanning directories matching `/@*/remote.yaml`.
- **Remove Remote**: `oops remote remove <name>` deletes the `/@<name>/` directory.

---

## Synchronous 1:1 Transfer Protocol (`oops sync`) [การซิงค์ไฟล์ตรงแบบ 1:1]

`oops sync` replaces string-based suffix stripping with direct directory mirroring:

- **Pull Operation (`oops sync pull -r <remote>`)**:
  - Connects to the host defined in `/@<remote>/remote.yaml`.
  - Downloads remote `.env` to local `/@<remote>/.env`.
  - Downloads remote `config/` overrides to local `/@<remote>/config/`.
- **Push Operation (`oops sync push -r <remote>`)**:
  - Connects to the host defined in `/@<remote>/remote.yaml`.
  - Uploads local `/@<remote>/.env` to remote `.env`.
  - Uploads local `/@<remote>/config/` to remote `config/`.
- **No Suffix Mangling**: Files retain identical names across local and remote environments.

---

## Grouping & Metadata Protocol [ระเบียบข้อมูลเมตาและการจัดกลุ่ม]

### Service Groups (`x-groups`) [การจัดกลุ่มผ่าน `x-groups`]
Service groupings are declared directly in root `compose.yaml` using Docker Compose's official extension specification (`x-*`):

```yaml
# compose.yaml (Project Root)
services:
  app:
    image: myapp:latest

x-groups:
  db:
    - mysql
    - redis
  monitoring:
    - prometheus
    - grafana
```

This prevents file proliferation at root while serving as a single source of truth for stack composition.

---

## Service Classification: Core vs. Environment-Specific [การจำแนกประเภทบริการ]

To balance local developer workstation responsiveness with production requirements, services are categorized into two tiers:

### Core Dependencies [บริการหลักที่จำเป็นต่อการรันแอพ]
- Declared in application compose files (`compose.<project>.yaml`) via native `depends_on` (e.g., `mysql`, `redis`) or explicit `x-services:` / `x-depends:`.
- Required for baseline application function. Always started during local execution (`oops up /<project>`).
- **Dependency Discovery**: When resolving `oops up /<project>`, `oops` automatically scans both `depends_on` blocks in container definitions and `x-services:` declarations, matching each against available `services/<service>.yaml` templates to construct the `-f` pipeline.

### Environment-Specific Services [บริการเสริมเฉพาะสภาพแวดล้อม เช่น Prod]
- Auxiliary tooling required on production servers (e.g., `grafana`, `prometheus`, log shippers) but unnecessary for everyday local feature testing.
- Declared in `/@<remote>/compose.yaml` using `x-services:`:
  ```yaml
  # @prod/compose.yaml
  x-services:
    - prometheus
    - grafana
  ```
- **Local Dev**: Remains inactive during standard local testing, conserving host RAM and CPU.
- **Production Deploy**: Automatically bundled and enabled when deploying to the target remote.

---

## CLI Routing Rules [กฎการกำหนดเส้นทางคำสั่ง CLI]

The CLI parses the target argument without requiring special module prefixes (no `_` prefix):

### Target Resolution Matrix [ตารางจับคู่คำสั่งและไฟล์]

| Command Pattern | Target Type | Resolution & `-f` Chain |
| :--- | :--- | :--- |
| `oops up` | Base Default | `-f compose.yaml [-f compose.local.yaml] up` |
| `oops up <service>` | Single Service | `-f compose.yaml -f services/<service>.yaml [-f compose.local.yaml] up <service>` |
| `oops up +<group>` | Service Group | Reads `x-groups.<group>` from `compose.yaml`, chains all member templates, and appends `[-f compose.local.yaml]` at the tail |
| `oops up /<project>` | Project Stack | `-f compose.yaml [dependencies] -f compose.<project>.yaml [-f compose.<project>.local.yaml] [-f compose.local.yaml] up` |

### Precedence Pipeline (Left-to-Right, Tail Wins) [ลำดับการซ้อนทับตามลำดับความสำคัญ]
When executing commands, `-f` arguments are assembled in strict evaluation order from base to localized overrides:
1. **Base Framework**: `-f compose.yaml` (Root anchor for working directory)
2. **Atomic Templates**: `-f services/<service>.yaml` (Pure templates matched from `depends_on` / `x-services`)
3. **Project Manifest**: `-f compose.<project>.yaml` (Project configuration)
4. **Project Local Override**: `-f compose.<project>.local.yaml` (Project-specific machine override)
5. **Global Machine Override**: `-f compose.local.yaml` (Global machine override — tailmost position)

---

## Deployment Overlay Engine (`oops deploy`) [ระบบส่งมอบผ่านการทับซ้อนไดเรกทอรี]

### 2-Step Direct SSH Streaming Pipeline [ขั้นตอนการส่งมอบตรงผ่าน SSH]
When deploying to a designated remote target (e.g., `oops deploy /myapp @prod` or `oops deploy @prod`):

- **Target Connection**: Reads destination SSH alias and path from `/@<remote>/remote.yaml`.
- **Step 1 — Base Synchronization**: Streams base manifests (`compose.yaml`, `services/`, `config/`), strictly excluding `*.local.yaml` and all `/@*` directories.
- **Step 2 — Overlay Application**: Streams the contents of `/@<remote>/` directly over the remote target root, strictly excluding `remote.yaml`.
- **Step 3 — Remote Execution**: Executes `docker compose up -d` over SSH.

### Dumb Server Model [แบบจำลองเซิร์ฟเวอร์ปลายทางแบบเรียบง่าย]
On the remote host:
- The target server receives a flattened, standard directory structure (`compose.yaml`, `services/`, `config/`, `.env`).
- Zero Git dependencies, zero bare repositories, and zero build daemons.
- The server executes native `docker compose up -d` cleanly and reliably.

---

## Spec Crystallization & Blast-Radius Audit [การตกผลึกเข้าสู่ Spec และพันธกิจการอัปเดตเอกสารทั้งหมด]

**MANDATORY PARITY INVARIANT [ข้อบังคับการรักษาสัจธรรมของระบบ]**:
Upon implementation of this protocol (GATE 2 / Execution), the contributor or AI agent **MUST NOT stop at code implementation**. All affected Living Specifications and architecture documents **MUST be comprehensively updated and synchronized** in the same changeset before completing the mission (GATE 3):

- **[docs/specs/cli.md](docs/specs/cli.md)**:
  - Rewrite discovery engine and command routing to match the new matrix (no `_` prefix, auto-detection of `services/<service>.yaml`, `x-groups:` parsing).
  - Update multi `-f` chaining pipeline, tail-chaining local overrides, and strict `.yaml` enforcement (deprecating all `.yml` and `docker-compose.*`).
- **[docs/specs/oopsbox.md](docs/specs/oopsbox.md)**:
  - Update taxonomy to the canonical triad (`services/`, `config/`, `data/`).
  - Rename `oopsbox.yml` and `oopsbox.yml.example` to `oopsbox.yaml` and `oopsbox.yaml.example`.
  - Update boot and active box invariants to match `.yaml` resolution.
- **[docs/specs/env-sync.md](docs/specs/env-sync.md)**:
  - Completely rewrite the specification from legacy Suffix Mapping Protocol (`.<remote>`) to the **1:1 Mirror Directory Sync Protocol (`/@<remote>/`)**.
  - Remove `stripRemoteSuffix` and `appendRemoteSuffix` logic.
- **[docs/specs/remote.md](docs/specs/remote.md)**:
  - Completely rewrite from Git Bare repositories and `post-receive` hooks to **Zero-Git Direct SSH Streaming**.
  - Document `/@<remote>/remote.yaml` connection schema and CLI commands (`oops remote add`, `list`, `remove`).
- **[README.md](../../README.md) & [oopsbox/README.md](../../oopsbox/README.md)**:
  - Update all directory trees, setup workflows, and example commands to use `.yaml` and `services/`.
- **[CONTEXT.md](../../CONTEXT.md)**:
  - Update Domain Dictionary and Ubiquitous Language definitions for `Remote Host`, `Service Template`, and `Environment Overlay`.
