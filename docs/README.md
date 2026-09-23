# Documentation Index & Architecture Taxonomy

This directory serves as the centralized knowledge repository for the system. Contributors and AI coding agents must strictly adhere to the system documentation taxonomy. Never create ad-hoc top-level directories under `docs/`.

## System Documentation Taxonomy

Every document in this directory answers a specific dimensional question:

- **`docs/architecture/` (WHERE - Macro Topology & Boundaries)**:
  - Answers: Where do components live, how do they communicate, and what are the system boundaries?
  - Content: High-level architectural diagrams, component interaction topologies, and system-wide data flow maps mirroring [ARCHITECTURE.md](../ARCHITECTURE.md).
  - Naming: Named with descriptive topic slug (e.g., `event-mesh.md`, `storage-topology.md`).
  - Template: Follows [docs/architecture/0000-template.md](architecture/0000-template.md).
  - Target: System architects, lead engineers, and onboarding contributors.

- **`docs/design/` (APPEARANCE - Global Design Tokens & Component Rules)**:
  - Answers: How do UI elements look and behave consistently across the entire application?
  - Content: Reusable component styling, design tokens, form patterns, table layouts, and accessibility standards mirroring [DESIGN.md](../DESIGN.md).
  - Naming: Named with component or pattern slug (e.g., `buttons.md`, `data-tables.md`).
  - Template: Follows [docs/design/0000-template.md](design/0000-template.md).
  - Target: Frontend engineers, UI designers, and AI coding agents.

- **`docs/development/` (HOW - Developer Manual, Workflows & Runbooks)**:
  - Answers: How do developers configure environments, run database migrations, execute tests, or perform deployments?
  - Content: Local setup guides, database migration runbooks, incident response playbooks, and operational workflows mirroring [CONTRIBUTING.md](../CONTRIBUTING.md).
  - Naming: Named with standard 4-digit numeric slug (e.g., `0001-local-setup.md`, `0002-db-migration.md`).
  - Template: Follows [docs/development/0000-template.md](development/0000-template.md).
  - Target: Developers, operators, and DevOps engineers.

- **`docs/specs/` (WHAT - Living Subsystem Specifications)**:
  - Answers: What does this feature do, what are its domain rules, state machines, API contracts, and database impacts?
  - Structure: Vertical slices combining domain invariants, endpoints, schemas, and bounded blast radiuses into a single cohesive specification per subsystem.
  - Governance: Strictly bound by the Code-Spec Parity Invariant. Changes to business logic or interfaces must update the corresponding spec in lockstep.
  - Template: Follows [docs/specs/0000-template.md](specs/0000-template.md).
  - Target: Software engineers and AI coding agents.

- **`docs/decisions/` (WHY - Architectural Decision Records)**:
  - Answers: Why did we choose this architectural approach over alternatives, and what were the evaluated trade-offs?
  - Content: Strategic macro decisions (such as database engine selection, partitioning schemes, or messaging frameworks).
  - Lifecycle & Naming:
    - Active Decisions: Named with standard numeric slug (e.g., `0001-postgresql.md`).
    - Inactive Decisions: Renamed with explicit status suffix (e.g., `0002-mongodb.deprecated.md` or `0004-session.superseded.md`).
  - Template: Follows [docs/decisions/0000-template.md](decisions/0000-template.md).
  - Target: All developers and AI agents evaluating architectural changes.

## Baseline Seeded Templates & Directory Structure

To keep the repository organized and structured:
- All core documentation directories (`docs/architecture/`, `docs/design/`, `docs/development/`, `docs/specs/`, and `docs/decisions/`) are seeded with active templates out of the box.
- Deep-dive topic documents are added to these directories on demand when extracting details from root index files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`).
- Never create empty directories with placeholder `.gitkeep` files if there is no immediate documentation content to commit.

## Anti-Drift Invariants for AI Coding Agents

- **No Ad-Hoc Directories**: Never create fragmented horizontal directories (such as `docs/domain/`, `docs/database/`, or `docs/api/`). Feature-specific domain rules, API contracts, and storage impacts must be consolidated inside `docs/specs/<feature>.md`.
- **Zero-Token Decision Filtering**: When querying `docs/decisions/`, inspect file names first. Always exclude files ending in `.deprecated.md` or `.superseded.md` from context ingestion.
- **Targeted Reading**: When implementing or debugging a feature, read only the matching specification in `docs/specs/<feature>.md` rather than loading unrelated documentation directories.
