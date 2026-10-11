---
trigger: always_on
description: "Naming conventions for files, environment variables, and ubiquitous domain language."
---

# Naming Conventions

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Add project-specific naming conventions to your project's `CONTEXT.md` only.

This document defines the naming conventions for configurations, variables, slugs, and domains across the project.

## Configuration Naming Principles

### Nested Hierarchy as Path Notation
- **Rule:** Nested configuration objects must act as a "Path" indicating the category, never constructed as a natural language sentence.
- **Rationale:** Path notation facilitates easy searching (grep/IntelliSense), keeps the codebase organized, and groups configurations logically.
- **Bad:** `CONFIG.WORKER.RUN_CLEANUP_EVERY_MS` (Constructed as a sentence)
- **Good:** `CONFIG.WORKER.CLEANUP_INTERVAL_MS` (Acts as a path)

### Noun to Modifier to Unit Pattern
- **Rule:** Key names must follow the strict order of **[Topic (Noun)] -> [Property/State (Modifier)] -> [Unit (if applicable)]**.
- **Rationale:** Ensures related variables are grouped together alphabetically and eliminates ambiguity regarding units of measurement.
- **Bad:** `MAX_TIMEOUT_MS` (Modifier precedes the noun)
- **Good:** `TIMEOUT_MAX_MS` (Topic: Timeout, Modifier: Max, Unit: MS)
- **Good:** `INACTIVITY_DAYS`, `RETRY_LIMIT_COUNT`

### Environment Variable Parity
- **Rule:** Variables in `.env` files (which are flat) must explicitly reflect the nested object structure in the code by using prefixes.
- **Rationale:** Allows developers to instantly map where an environment variable is injected within the system architecture.
- **Example:** `DATABASE_POOL_SIZE` in `.env` strictly maps to `CONFIG.DATABASE.POOL_SIZE`.

### Compound Words as Single Identity
- **Rule:** Compound words or single-entity concepts within the system's context must NOT be separated by underscores (`_`), even if written separately in natural language.
- **Rationale:** Reduces verbosity and preserves the concept as a single logical entity rather than a noun with a modifier.
- **Bad:** `AUTO_START_DELAY_MS` (Makes "Auto" look like a modifier for "Start")
- **Good:** `AUTOSTART_DELAY_MS` ("Autostart" is treated as a single noun)
- **Good:** `WEBSOCKET_PORT` (Not `WEB_SOCKET_PORT`), `FILENAME` (Not `FILE_NAME`)

## Slug & Document Path Notation

### Path Notation over Sentence-Like Slugs
- **Rule:** Slugs for files (`docs/**`), ADRs, living specs, runbooks, and Git branches must use `kebab-case` structured as a hierarchical path: **`[Domain/Topic]-[Modifier/Subtopic]-[Detail/Entity]`**. Never construct slugs as natural language sentences.
- **Rationale:** Groups related files alphabetically, enables instant pattern searches (`grep`, glob), and eliminates deeply nested folder structures while preserving structural depth for AI context ingestion.
- **Bad:** `how-to-login-with-google.md`, `fix-the-broken-database-timeout.md`
- **Good:** `auth-oauth-google.md`, `db-timeout-retry.md`

### Slug Formatting Invariants
- **Lowercase & Hyphens Only**: Strictly lowercase `a-z`, digits `0-9`, and single hyphens `-`. No uppercase, spaces, or underscores.
- **Concise Scope**: 2–5 words focusing on domain intent. Omit conversational filler words (`the`, `a`, `and`, `how-to`).
- **Numeric & Timestamp Prefixes**: When sequential order or identifier tracking is required (ADRs in `docs/adr/`), use a 4-digit zero-padded prefix: `XXXX-<slug>.md` (e.g., `0001-project-identity.md`). **Explorations in `docs/exp/`** use a lightweight timestamped sequence prefix: `YYMM-<seq>-<slug>.md` (e.g., `2610-1-shadow-merge.md`) with a corresponding YAML `id` (`exp-2610-1`) to track age and prevent long-term WIP stagnation. A permanent 4-digit sequence is assigned only upon finalization/crystallization into `docs/adr/`.
- **Exploration Sequence Invariants (Uniqueness & Immutability)**:
  - **Monthly Reset**: Sequence index `<seq>` resets to `1` at the beginning of each calendar month (e.g., `2610-1`, `2610-2`, then `2611-1` next month).
  - **Never Reused (Monotonic)**: An issued ID within the month is never reused or recycled under any circumstances, even if the exploration is cancelled, rejected, or deleted. Always allocate `max(seq) + 1`.
  - **Slug Decoupling (ID Immutability)**: Slugs may be updated or refined over time without altering the assigned numeric prefix or YAML `id`.
  - **Permanent Retirement**: When an exploration is crystallized (into an ADR/Spec) or archived, its ID is permanently retired and never reassigned to subsequent topics.
- **Directory-Scoped Filename Hygiene (DRY)**: Filenames do NOT repeat the directory name prefix (e.g., use `docs/exp/2610-1-shadow-merge.md`, not `docs/exp/exp-2610-1-...`). The directory establishes the domain boundary; the YAML `id` carries the full global unique identifier.

### Document ID & Taxonomy Dictionary Table

All documents and transient issues must follow this centralized identifier convention:

| Domain | Directory | Filename Format | YAML `id` | Markdown / Chat Reference |
| :--- | :--- | :--- | :--- | :--- |
| **Architectural Decisions** | `docs/adr/` | `XXXX-<slug>.md` | `adr-XXXX` (e.g., `adr-0001`) | `adr-0001` or `[adr-0001](docs/adr/0001-*.md)` |
| **Explorations** | `docs/exp/` | `YYMM-<seq>-<slug>.md` | `exp-YYMM-<seq>` (e.g., `exp-2610-1`) | `exp-2610-1` or `[exp-2610-1](docs/exp/2610-1-*.md)` |
| **How-to Guides & Wiki** | `docs/howto/` | `<slug>.md` | `<slug>` (e.g., `local-setup`) | `howto/local-setup` |
| **Transient Issues** | `.scratch/<task>/issues/` | `XXXX-<slug>.md` | `issue-XXXX` (e.g., `issue-0001`) | `issue-0001` or `depends_on: ["issue-0001"]` |
| **Living Specifications** | `docs/specs/` | `<feature>.md` | `<feature>` (e.g., `auth`) | `spec/auth` |
| **Architecture Deep-Dives** | `docs/architecture/` | `<topic>.md` | `<topic>` (e.g., `topology`) | `arch/topology` |
| **Design Specifications** | `docs/design/` | `<topic>.md` | `<topic>` (e.g., `buttons`) | `design/buttons` |

## Domain & Intent-Based Naming

- **Ubiquitous Language Invariant (Domain Alignment)**: All system layers—including database columns, API parameters, variable names, and class definitions—MUST strictly use the exact vocabulary defined in `CONTEXT.md` (Domain Dictionary). Developers and AI agents are prohibited from inventing synonyms or new terms for established domain concepts.
- **Intent-Based Naming**: Variables and functions reflect domain intent, not mechanism. Generic placeholders (`data`, `temp`, `helper`, `manager`, `process`) are avoided.
- **Shared Technical Utilities & Tools**: Shared stateless tools, formatting engines, ID generators, or cryptographic helpers shared project-wide ARE PERMITTED under `pkg/utils/` or `shared/tools/`. They MUST be named specifically after their technical responsibility (e.g., `id-generator`, `number-formatter`, `date-utils`), avoiding generic catch-all names like `misc` or `stuff`.
- **Domain Separation Invariant**: Shared utilities MUST remain strictly stateless and technical. Placing domain-specific business rules, entity logic, or database access inside shared utility files is strictly prohibited.
- **Boolean Predicates**: Boolean variables and functions use clear prefixes (`is_active`, `has_access`, `can_modify`, `should_retry`).

## The Status Dictionary (Zero-Token Status Filtering)

- **Rule**: Filename extensions may include a lifecycle postfix immediately preceding `.md` (`XXXX-<slug>.<postfix>.md`) to communicate document and task state for instant zero-token filtering via `ls` or globbing without reading file contents.
- **Stable Numeric Prefix**: The leading `XXXX-` sequence identifier MUST NOT change when a status postfix is transitioned (e.g., active to `.superseded.md`). Note: Explorations use `exp-YYMM-<seq>-<slug>.md`.

### Status Dictionary Table

| Postfix | Target Domain | Definition & Usage |
| :--- | :--- | :--- |
| `[none]` | All | Active, ongoing, accepted, or pending state. |
| `.done.md` | Task Issues | The task has been completed, verified, and committed. |
| `.blocked.md` | Task Issues | The task cannot proceed due to a prerequisite or external blocker. |
| `.deferred.md` | Task Issues | The task is postponed to a future milestone. |
| `.dropped.md` | Task Issues | The task is cancelled or "won't do". |
| `.superseded.md` | ADRs | The decision was active but is replaced by a newer ADR. |
| `.deprecated.md` | ADRs | The decision is retired without a direct replacement. |
| `.rejected.md` | ADRs | The proposal was evaluated but formally rejected. Kept for historical record. |
| *(Archive)* | Explorations | Active exploration records (`docs/exp/exp-YYMM-<seq>-<slug>.md`) that finalize with significant reasoning are moved to `docs/archived/<slug>.md` to freeze as a knowledge system. |
