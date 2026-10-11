---
id: "exp-2610-1"
title: "Compose Taxonomy, Discovery & Routing Protocol"
status: active
tags: [exploration, discovery, docker, compose, routing]
crystallization:
  target: spec
synapses: ["docs/specs/cli.md"]
---

# exp-2610-1: Compose Taxonomy, Discovery & Routing Protocol

## Context
`oops` acts as a smart ergonomic wrapper around `docker compose`. A core goal is to eliminate the need to manually type long `-f compose.yml -f compose.override.yml` commands.

Furthermore, there is a clear distinction between local development (`oopsbox`) and production servers. Local development often requires port forwards, host volume mounts, and debug settings that should never reach production.

## Decision
1. **Taxonomy & File Naming Conventions**:
   - `compose.yml`: Base configuration (Tracked in Git, shared across all environments).
   - `compose.<project>.yaml`: The core project-level composition.
   - `compose.<project>@<remote>.yaml` (e.g., `compose.myapp@prod.yaml`): Specific remote/environment override for that project (Tracked in Git).
   - `*.local.yaml` (e.g., `compose.local.yaml`, `compose.myapp.local.yaml`): Machine-specific overrides. Ignored by Git.

2. **Auto-Discovery & Execution Merge (Run-Time)**:
   - When running `oops up` (whether locally or on a production server), `oops` will determine the target root file (e.g., `compose.myapp.yaml`).
   - It will **always** search for a matching file with `.local.yaml` appended (e.g., `compose.myapp.local.yaml`) and append it to the `-f` chain.
   - **Example `-f` chain**: `-f compose.myapp.yaml -f compose.myapp.local.yaml`
   - **Purpose**: Allows any machine (developer laptop or specific production server) to have untracked, machine-specific overrides without affecting the Git repository.

4. **Deployment Routing & Intelligent Sync (Remote Production)**:
   - **Namespace-Guarded Routing**: To prevent accidental cross-project deployments, the `remote` target name is strictly derived using the `@` symbol (`<project>@<env|server>`).
     - **Default Fallback**: `oops deploy /project-a` implicitly deploys to `project-a@prod`.
     - **Explicit Override**: `oops deploy /project-a@server2` deploys directly to `project-a@server2`.
   - **Intelligent File Syncing (No Pre-Compilation)**: `oops` does **not** compile the files into a flat YAML. Instead, it acts as a smart bundler:
     1. It reads the target file (e.g., `compose.myapp@prod.yaml`).
     2. It parses the YAML to discover which files are referenced under the `include:` directive (e.g., `modules/mysql.yaml`).
     3. It transfers the target file AND the required module files to the remote server, preserving the `modules/` directory structure.
     4. On the server, the target file (`compose.myapp@prod.yaml`) is **renamed to `compose.yml`**.
   - **CRITICAL - No Local Files in Deploy**: `*.local.yaml` files are **strictly excluded** from the sync step.
   - **Dumb Server Model**: The production server simply receives a standard `compose.yml` and a `modules/` folder, then runs `docker compose up -d`. The relative paths (`../`) inside the modules continue to work perfectly on the server.

## Consequences
- Highly flexible taxonomy allowing broad base configs alongside hyper-specific overrides per organization, project, or server.
- `*.local.yaml` guarantees that machine-specific overrides (like a developer's open ports, or a server admin's manual tweaks) never leak to Git and are never overwritten by automated deployments.
- `compose.<project>@<remote>.yaml` allows a clean layer to define server-specific baselines without needing an overly complex or broad `compose.remote.yml` file.
- **Shift-Left Complexity**: Complexity is handled during the deploy/sync phase by `oops`. The production environment remains exceptionally clean, robust, and easy to debug since the server just sees a standard `compose.yml` referencing local modules.
---

## Part 2: Modular Compose Topology & Prefix Routing

### Context
Instead of managing a monolithic `compose.yml` file, we are exploring a highly modular approach where configurations are broken down into discrete layers: Services (Modules), Groups, and Projects. The `oops` CLI will use specific prefix symbols to identify and route commands to these layers.

### Decision (Concept)
We define three layers of Compose files using a strict prefix naming convention:

1. **Service Modules / System Services (Pending Decision: Root Prefix vs. Subdirectory)**
   - **Purpose:** Pure, atomic building blocks (e.g., a database, a cache, a specific backend) that function as local or system-level infrastructure.
   - *Note: The exact placement and naming of these modules are marked for future evaluation. Two approaches are currently drafted:*

   - **Option A (Original Root Prefix - Refined):**
     - **Naming:** `_compose.<service>.yaml` (kept in the project root).
     - **Examples:** `_compose.mysql.yaml`, `_compose.redis.yaml`
     - **Why `_compose`?**: Because `oops` only auto-scans for files starting with `compose*.yaml`, files prefixed with `_` naturally bypass the `oops` discovery engine. They exist solely to be `include`d.

   - **Option B (Directory-based):**
     - **Naming:** `modules/<service>.yaml` (The `compose._` prefix is removed, relying on the folder structure).
     - **Examples:** `modules/mysql.yaml`, `modules/redis.yaml`

   - **Execution Rule:** `oops` will only target these files directly if the developer explicitly uses the `_` prefix in the CLI (`oops up _mysql`). Otherwise, they are meant to be pulled in via Docker Compose's `include` directive from root-level project/group files.
   - **Path Resolution Consequence (If using Option B):** Because they are loaded via `include` from a subdirectory, any relative volume mounts inside a module must use `../` to target the project root (e.g., `../volumes/mysql:/var/lib/mysql`). If Option A is chosen, `./` works perfectly.

2. **Group Layer (Prefix: `+`)**
   - **Purpose:** Aggregation layer. Used to combine multiple Service Modules together (e.g., all database-related services). It dictates *which* services are run together.
   - **Naming Convention:** `compose.+<group>.yaml`
   - **Examples:** `compose.+db.yaml`
   - **CLI Usage:** `oops up +db`

3. **Project Layer (Prefix: `/`)**
   - **Purpose:** The highest level of orchestration representing a complete application or namespace.
   - **Naming Convention:** `compose.<project>.yaml`
   - **Examples:** `compose.myapp.yaml`
   - **CLI Usage:** `oops up /myapp`

### CLI Command Routing Rules
The CLI parses the first argument to determine which root file to load. Any subsequent arguments are passed to Docker Compose as target services.

1. **`oops up [<service>]`**
   - **Behavior:** Always uses the base `compose.yml` as the root.
   - **Example:** `oops up mysql` -> `docker compose -f compose.yml up mysql`

2. **`oops up _<module>`**
   - **Behavior:** Explicitly targets a Service Module file (`_compose.<module>.yaml` or `modules/<module>.yaml`) as the root. Useful for spinning up a single service standalone without needing a group or project to include it.
   - **Example:** `oops up _mysql` -> `docker compose -f _compose.mysql.yaml up`

3. **`oops up +<group> [<service>]`**
   - **Behavior:** Uses the group file `compose.+<group>.yaml` as the root. Commonly used to run specific developer toolsets.
   - **Example:** `oops up +db` -> `docker compose -f compose.+db.yaml up`
   - **Example:** `oops up +db redis` -> `docker compose -f compose.+db.yaml up redis`

4. **`oops up /<project> [<service>]`**
   - **Behavior:** Uses the project file `compose.<project>.yaml` as the root to run the project's environment and services.
   - **Example:** `oops up /myapp` -> `docker compose -f compose.myapp.yaml up`
   - **Example:** `oops up /myapp api` -> `docker compose -f compose.myapp.yaml up api`

### Consequences
- **High Reusability:** Service definitions like `modules/mysql.yaml` can be reused across different groups and projects without duplication.
- **Strict Root-Only `-f` Pipeline:** `oops` will only scan and append `-f` flags for files located in the project root (`compose*.yaml`). This drastically simplifies the CLI orchestrator's logic.
- **Native Alignment:** This forces full reliance on Docker Compose's native `include` feature for module assembly. Group files (`compose.+db.yaml`) and Project files (`compose.<project>.yaml`) will explicitly `include` what they need.
- **Semantic CLI:** The prefix inherently tells the developer what level of the stack they are interacting with, and the routing logic is completely predictable.
