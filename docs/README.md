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
    - Inactive Decisions: Renamed with explicit status suffix conforming to the Status Dictionary in [.agents/rules/jarn-naming.md](../.agents/rules/jarn-naming.md) (e.g., `0002-mongodb.deprecated.md` or `0004-session.superseded.md`).
  - Template: Follows [.agents/templates/docs/adr.md](../.agents/templates/docs/adr.md).
  - Target: All developers and AI agents evaluating architectural changes.

- **`docs/drafts/` (IDEATION - Active Brainstorming & Planning)**:
  - Answers: What are we currently planning, discussing, or exploring before committing to a final specification?
  - Content: Raw ideas, active discussions with AI, temporary schemas, and unstructured logic.
  - Naming: Named with topic slug, strictly NO numeric prefix (e.g., `multi-server-deploy.md`).
  - Target: Developers and AI agents in GATE 1 (Consultation Phase).

- **`docs/archived/` (HISTORY - Preserved Reasoning)**:
  - Answers: What were the original raw discussions and abandoned concepts that led to the finalized specification?
  - Content: Frozen drafts that contain valuable historical context or "why" reasoning not fully captured in the final ADR or Spec.
  - Lifecycle: Migrated from `docs/drafts/`. Must include `resolved_to: ...` in YAML frontmatter pointing to the finalized document.

## The Draft Lifecycle & AI Collaboration Flow

To optimize AI token consumption, preserve architectural reasoning, and maintain a clean Git history, all document drafting must follow this workflow:

1. **Initiate (Drafting)**: Create a new `.md` file in `docs/drafts/` (e.g., `docs/drafts/my-feature.md`). Avoid using AI web-based "Artifact Modes" for long documents, as they consume excessive tokens by regenerating entire files on every minor edit.
2. **Iterate (Targeted Patching)**: Open the draft in your IDE. For minor wording adjustments, edit the file manually. For logic or structural changes, highlight or copy snippets into the chat and instruct the AI. The AI will use targeted file patches to surgically edit only the requested lines, saving massive Output Tokens.
3. **Finalize (Promotion & Archival)**: 
   - Once consensus is reached, the formal document is generated in the correct taxonomy folder (`docs/specs/`, `docs/adr/`, etc., receiving a numeric prefix if applicable).
   - The original draft is moved to `docs/archived/` if it contains valuable unextracted reasoning or historical context (The Non-Subtractive Principle). If it was merely temporary notes, it can be deleted.

## Baseline Centralized Templates & Directory Structure

To keep the repository clean and avoid mixing placeholder templates with actual project files:
- All official document and task templates reside centralized under `.agents/templates/` (`.agents/templates/docs/` and `.agents/templates/scratch/`).
- Subdirectories under `docs/` contain strictly production project documentation.
- Deep-dive topic documents are added to these directories on demand when extracting details from root index files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`).

## Anti-Drift Invariants for AI Coding Agents

- **No Ad-Hoc Directories**: Never create fragmented horizontal directories (such as `docs/domain/`, `docs/database/`, or `docs/api/`). Feature-specific domain rules, API contracts, and storage impacts must be consolidated inside `docs/specs/<subsystem>.md`.
- **Current System Truth over Task Delta**: Never create transient feature-request specs (e.g. `docs/specs/add-oauth.md`). In-flight tasks live in implementation plans or task trackers; `docs/specs/<subsystem>.md` is updated in place upon feature completion.
- **Zero-Token Decision Filtering**: When querying `docs/adr/`, inspect file names first. Always exclude files ending with status dictionary postfixes (e.g., `.deprecated.md`, `.superseded.md`) from context ingestion.
- **Targeted Reading**: When implementing or debugging a feature, read only the matching specification in `docs/specs/<subsystem>.md` rather than loading unrelated documentation directories.
