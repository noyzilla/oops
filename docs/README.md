# Documentation Index & Architecture Taxonomy

This directory serves as the centralized knowledge repository for the system. Contributors and AI coding agents must strictly adhere to the system documentation taxonomy. Never create ad-hoc top-level directories under `docs/`.

## System Documentation Taxonomy

Every document in this directory answers a specific dimensional question:

- **`docs/architecture/` (WHERE - Macro Topology & Boundaries)**:
  - Answers: Where do components live, how do they communicate, and what are the system boundaries?
  - Content: High-level architectural diagrams, component interaction topologies, and system-wide data flow maps mirroring [ARCHITECTURE.md](../ARCHITECTURE.md).
  - Target: System architects, lead engineers, and onboarding contributors.

- **`docs/design/` (APPEARANCE - Global Design Tokens & Component Rules)**:
  - Answers: How do UI elements look and behave consistently across the entire application?
  - Content: Reusable component styling, design tokens, form patterns, table layouts, and accessibility standards mirroring [DESIGN.md](../DESIGN.md).
  - Target: Frontend engineers, UI designers, and AI coding agents.

- **`docs/development/` (HOW - Developer Manual, Workflows & Runbooks)**:
  - Answers: How do developers configure environments, run database migrations, execute tests, or perform deployments?
  - Content: Local setup guides, database migration runbooks, incident response playbooks, and operational workflows mirroring [CONTRIBUTING.md](../CONTRIBUTING.md).
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

## On-Demand Directory Creation (Just-In-Time)

To keep the repository clean and avoid empty directories:
- Subdirectories `docs/architecture/`, `docs/design/`, and `docs/development/` are created on demand when extracting deep-dive topics from their parent root files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`).
- Only directories seeded with active templates (`docs/specs/` and `docs/decisions/`) exist out of the box.
- Never create empty directories with placeholder `.gitkeep` files if there is no immediate documentation content to commit.

## Anti-Drift Invariants for AI Coding Agents

- **No Ad-Hoc Directories**: Never create fragmented horizontal directories (such as `docs/domain/`, `docs/database/`, or `docs/api/`). Feature-specific domain rules, API contracts, and storage impacts must be consolidated inside `docs/specs/<feature>.md`.
- **Zero-Token Decision Filtering**: When querying `docs/decisions/`, inspect file names first. Always exclude files ending in `.deprecated.md` or `.superseded.md` from context ingestion.
- **Targeted Reading**: When implementing or debugging a feature, read only the matching specification in `docs/specs/<feature>.md` rather than loading unrelated documentation directories.
