# Documentation Index & Architecture Taxonomy

This directory serves as the centralized knowledge repository for the system. Contributors and AI coding agents must strictly adhere to the system documentation taxonomy. Never create ad-hoc top-level directories under `docs/`.

## System Documentation Taxonomy

Every document in this directory answers a specific dimensional question:

- **`docs/architecture/` (WHERE - Macro Topology & Boundaries)**:
  - Answers: Where do components live, how do they communicate, and what are the system boundaries?
  - Content: High-level architectural diagrams, component interaction topologies, and system-wide data flow maps mirroring [ARCHITECTURE.md](../ARCHITECTURE.md).
  - Naming: Named with descriptive topic slug (e.g., `event-mesh.md`, `storage-topology.md`).
  - Template: Follows [.agents/templates/docs/architecture.md](../.agents/templates/docs/architecture.md).
  - Target: System architects, lead engineers, and onboarding contributors.

- **`docs/design/` (APPEARANCE - Global Design Tokens & Component Rules)**:
  - Answers: How do UI elements look and behave consistently across the entire application?
  - Content: Reusable component styling, design tokens, form patterns, table layouts, and accessibility standards mirroring [DESIGN.md](../DESIGN.md).
  - Naming: Named with component or pattern slug (e.g., `buttons.md`, `data-tables.md`).
  - Template: Follows [.agents/templates/docs/design.md](../.agents/templates/docs/design.md).
  - Target: Frontend engineers, UI designers, and AI coding agents.

- **`docs/development/` (HOW - Developer Manual, Workflows & Runbooks)**:
  - Answers: How do developers configure environments, run database migrations, execute tests, or perform deployments?
  - Content: Local setup guides, database migration runbooks, incident response playbooks, and operational workflows mirroring [CONTRIBUTING.md](../CONTRIBUTING.md).
  - Naming: Named with standard 4-digit numeric slug (e.g., `0001-local-setup.md`, `0002-db-migration.md`).
  - Template: Follows [.agents/templates/docs/development.md](../.agents/templates/docs/development.md).
  - Target: Developers, operators, and DevOps engineers.

- **`docs/specs/` (WHAT - Living Subsystem Specifications)**:
  - Answers: What does this subsystem do today, what are its domain rules, state machines, API contracts, and database impacts?
  - Core Principle (Spec is Law): Living specifications represent **Current System Truth**, never transient feature requests or task backlogs (e.g., `docs/specs/cli.md`, `docs/specs/webhook.md`). Code implementations must never conflict with the active spec.
  - Structure: Vertical slices combining domain invariants, endpoints, schemas, and bounded blast radiuses into a single cohesive specification per subsystem.
  - Governance: Strictly bound by the Code-Spec Parity Invariant. When a new capability or behavioral change is completed, the corresponding subsystem spec is updated in place to reflect the new system truth.
  - Template: Follows [.agents/templates/docs/spec.md](../.agents/templates/docs/spec.md).
  - Target: Software engineers and AI coding agents.

- **`docs/adr/` (WHY - Architectural Decision Records)**:
  - Answers: Why did we choose this architectural approach over alternatives, and what were the evaluated trade-offs?
  - Content: Strategic macro decisions (such as database engine selection, partitioning schemes, or messaging frameworks).
  - Lifecycle & Naming:
    - Active Decisions: Named with standard numeric slug (e.g., `0001-postgresql.md`).
    - Inactive Decisions: Renamed with explicit status suffix (e.g., `0002-mongodb.deprecated.md` or `0004-session.superseded.md`).
  - Template: Follows [.agents/templates/docs/adr.md](../.agents/templates/docs/adr.md).
  - Target: All developers and AI agents evaluating architectural changes.

## Baseline Centralized Templates & Directory Structure

To keep the repository clean and avoid mixing placeholder templates with actual project files:
- All official document and task templates reside centralized under `.agents/templates/` (`.agents/templates/docs/` and `.agents/templates/scratch/`).
- Subdirectories under `docs/` contain strictly production project documentation.
- Deep-dive topic documents are added to these directories on demand when extracting details from root index files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`).

## Anti-Drift Invariants for AI Coding Agents

- **No Ad-Hoc Directories**: Never create fragmented horizontal directories (such as `docs/domain/`, `docs/database/`, or `docs/api/`). Feature-specific domain rules, API contracts, and storage impacts must be consolidated inside `docs/specs/<subsystem>.md`.
- **Current System Truth over Task Delta**: Never create transient feature-request specs (e.g. `docs/specs/add-oauth.md`). In-flight tasks live in implementation plans or task trackers; `docs/specs/<subsystem>.md` is updated in place upon feature completion.
- **Zero-Token Decision Filtering**: When querying `docs/adr/`, inspect file names first. Always exclude files ending in `.deprecated.md` or `.superseded.md` from context ingestion.
- **Targeted Reading**: When implementing or debugging a feature, read only the matching specification in `docs/specs/<subsystem>.md` rather than loading unrelated documentation directories.
