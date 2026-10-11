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

- **`docs/howto/` (HOW - Procedural How-to Guides, Operations & Wiki)**:
  - Answers: How do developers and operators configure environments, run database migrations, execute tests, deploy, or troubleshoot incidents?
  - Content: Actionable procedures, onboarding guides, database runbooks, deployment playbooks, and operations wiki.
  - Structure & TOC: Pure slug (`<slug>.md`) navigation. Root `README.md` acts as master TOC. High-density topics expand into sub-directories (`docs/howto/<subtopic>/`) with a local `README.md`.
  - Template: Follows [.agents/templates/docs/howto.md](../.agents/templates/docs/howto.md).
  - Target: Developers, operators, DevOps engineers, and AI coding agents.

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

- **`docs/exp/` (EXPLORATION - Discovery, Spike & Technical Brainstorming)**:
  - Answers: What unknown, architectural challenge, or discovery objective are we exploring before committing to a formal specification?
  - Content: Discovery notes, technical spikes, architectural trade-offs, and evolving reasoning.
  - Naming: Named with timestamp-indexed prefix `YYMM-<seq>-<slug>.md` (e.g., `2610-1-shadow-merge.md`).
  - Sequence Rules: Sequence resets each month; IDs are monotonic and never reused; slugs can evolve without mutating IDs; retired upon crystallization or rejection.
  - Template: Follows [.agents/templates/docs/exp.md](../.agents/templates/docs/exp.md).
  - Target: Developers and AI agents in GATE 1 (Consultation / Spike Phase).

- **`docs/archived/` (HISTORY - Preserved Reasoning)**:
  - Answers: What were the original raw discussions and abandoned concepts that led to the finalized specification?
  - Content: Frozen exploration records that contain valuable historical context or "why" reasoning not fully captured in the final ADR or Spec.
  - Lifecycle: Migrated from `docs/exp/`. Must include `crystallization.resolved_to: ...` in YAML frontmatter pointing to the finalized document.

## Document ID & Taxonomy Dictionary Table

All documents and transient tasks adhere to this unified identifier and naming convention:

| Domain | Directory | Filename Format | YAML `id` | Markdown / Chat Reference |
| :--- | :--- | :--- | :--- | :--- |
| **Architectural Decisions** | `docs/adr/` | `XXXX-<slug>.md` | `adr-XXXX` (e.g., `adr-0001`) | `adr-0001` or `[adr-0001](docs/adr/0001-*.md)` |
| **Explorations** | `docs/exp/` | `YYMM-<seq>-<slug>.md` | `exp-YYMM-<seq>` (e.g., `exp-2610-1`) | `exp-2610-1` or `[exp-2610-1](docs/exp/2610-1-*.md)` |
| **How-to Guides & Wiki** | `docs/howto/` | `<slug>.md` | `<slug>` (e.g., `local-setup`) | `howto/local-setup` |
| **Transient Issues** | `.scratch/<task>/issues/` | `XXXX-<slug>.md` | `issue-XXXX` (e.g., `issue-0001`) | `issue-0001` or `depends_on: ["issue-0001"]` |
| **Living Specifications** | `docs/specs/` | `<feature>.md` | `<feature>` (e.g., `auth`) | `spec/auth` |
| **Architecture Deep-Dives** | `docs/architecture/` | `<topic>.md` | `<topic>` (e.g., `topology`) | `arch/topology` |
| **Design Specifications** | `docs/design/` | `<topic>.md` | `<topic>` (e.g., `buttons`) | `design/buttons` |

## The Exploration Lifecycle & AI Collaboration Flow

To optimize AI token consumption, preserve architectural reasoning, and maintain a clean Git history, all document discovery must follow this workflow:

1. **Initiate (Exploration)**: Create a new `.md` file in `docs/exp/` following the naming convention `YYMM-<seq>-<slug>.md` (e.g., `docs/exp/2610-1-shadow-merge.md`). Avoid using AI web-based "Artifact Modes" for long documents, as they consume excessive tokens by regenerating entire files on every minor edit.
2. **Iterate (Targeted Patching)**: Open the exploration file in your IDE. For minor wording adjustments, edit the file manually. For logic or structural changes, highlight or copy snippets into the chat and instruct the AI. The AI will use targeted file patches to surgically edit only the requested lines, saving massive Output Tokens.
3. **Finalize (Crystallization & Archival)**:
   - Once consensus is reached, the formal document is generated in the correct taxonomy folder (`docs/specs/`, `docs/adr/`, etc., receiving a permanent numeric prefix if applicable).
   - The original exploration file is moved to `docs/archived/` if it contains valuable unextracted reasoning or historical context (The Non-Subtractive Principle). If it was merely temporary notes, it can be deleted.

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
