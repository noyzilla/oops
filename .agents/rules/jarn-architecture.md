---
trigger: always_on
description: "Ecosystem-native lifecycle, documentation topology, and configuration architecture."
---

# Architecture & Lifecycle Standards

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Document project-specific architectural decisions in `docs/adr/` and topology in `docs/architecture/` only.

This document establishes the architectural principles, documentation topology, and native ecosystem lifecycles for the project.

## Domain & Data-First Principles

### Data-First & Contract-First Design
- **Rule**: Architectural designs must lock down data schemas, type definitions, and API contracts before any behavioral logic is implemented.
- **Rationale**: Ensures the foundation is solid and agreed upon, preventing extensive rework caused by shifting data structures mid-implementation.

### State Machine Invariant
- **Rule**: Complex business entities must be modeled using explicit state machines (e.g., `Draft` -> `Pending` -> `Active`) rather than ad-hoc boolean flags (e.g., `is_draft`, `is_active`).
- **Rationale**: Prevents impossible states, implicit logic branching, and data inconsistency.

### Domain Isolation (Model the Domain)
- **Rule**: Business rules and domain logic must be strictly isolated into pure functions or modules (e.g., `src/domain/`). They must NOT be coupled with database ORMs, HTTP frameworks, or UI layers.
- **Rationale**: Protects core business logic from technical infrastructure churn and enables fast, isolated unit testing.
## Ecosystem-Native Lifecycle Contract

Rather than forcing an unnatural wrapper layer across diverse ecosystems, every project must implement standard lifecycles using the **native tooling of its target programming language**.

Every repository must define clear entrypoints for the following operational lifecycles:

- **Setup / Install**: Prepare toolchains, package dependencies, and local environment configs.
- **Unit & Integration Test**: Execute automated unit, integration, and contract tests using language-native test runners.
- **UI & Visual Verification**: Verify user-facing frontend surfaces using ecosystem-native E2E runners (e.g., Playwright, Cypress, Selenium, Rod, or native browser automation) or empirical Visual Verification (screenshots/recordings captured via browser tools).
- **Lint & Style Check**: Check code formatting, syntax conventions, and run static analyzers.
- **Build**: Compile, typecheck, or bundle the project artifacts.
- **Run / Dev**: Launch the development process, service, or interactive session.
- **Clean**: Remove temporary artifacts, test caches, and compiled builds.

Document the exact commands for these lifecycles directly in `AGENTS.md` (for automated agents) and `CONTRIBUTING.md` (for human contributors).

## Configuration Architecture

### Global vs. Scoped Placement
- **Rule:** System-wide or cross-module settings must reside at the root level (e.g., `PORT`, `PROXY_URL`, `DEBUG`). Domain-specific settings must be encapsulated within their respective domain objects (e.g., `DATABASE`, `CACHE`, `MAILER`).
- **Bad:** Hiding `PORT` inside `CONFIG.SERVER.PORT` when it is globally required.
- **Good:** `CONFIG.PORT` for global settings and `CONFIG.CACHE.TTL_SECONDS` for scoped settings.

### Strict Separation of Config vs. Internal Constants
- **Rule:** Configuration and `.env` files are exclusively for values that environment administrators can alter. Internal engineering constants (e.g., internal folder names, cache key prefixes like `'user_session'`) must NEVER be exposed as configurations.
- **Rationale:** Business-agnostic internal values should be hardcoded or defined as constants in the lowest applicable layer to reduce indirection, prevent config bloat, and avoid catastrophic user misconfigurations.

## The Mirror Index Pattern (Document Splitting Protocol)

To keep root-level documentation clean and prevent context bloat:

### Root File Invariants
- Root documentation files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`) must remain lean master indexes and executive summaries.
- Root files must not exceed approximately 200 lines or hold deep implementation specs.

### Subdirectory Extraction Triggers
Extract or create detailed documentation under `docs/<name>/` when:
- Detailing a specific subsystem, isolated module, database schema, or runbook.
- Content exceeds 150-200 lines or contains extensive schemas and tables.
- Information is reference-only for specific domain workflows.

### Target Mapping Convention
Always organize documentation under `docs/` according to the system documentation taxonomy defined in `docs/README.md`:
- Subsystem and feature living specifications go into `docs/specs/<feature>.md` (vertical slices combining domain rules, APIs, and database impacts)
- Macro topologies and architectural deep-dives go into `docs/architecture/<topic>.md` (mirroring `ARCHITECTURE.md`)
- Global design tokens and UI component guides go into `docs/design/<topic>.md` (mirroring `DESIGN.md`)
- Developer onboarding, runbooks, and operational workflows go into `docs/development/<id>-<slug>.md` (mirroring `CONTRIBUTING.md`)
- Macro architectural decision records go into `docs/adr/<id>-<slug>.md` (with explicit lifecycle naming conforming to the Status Dictionary in `jarn-naming.md` for zero-token AI filtering)
- Unclassified working knowledge, brainstorming, and pending decisions go into `docs/drafts/<slug>.md`. Once finalized, the formal document (Spec/ADR) is generated in its respective folder. The draft itself MUST NOT be destroyed if it contains valuable reasoning; instead, move it to `docs/archived/<slug>.md` to permanently preserve the historical "why".

### Two-Way Linking Requirement
Whenever a sub-document is created under `docs/<name>/`:
- Add a bullet point and link under the deep-dive index section of the root parent file.
- Add a backlink to the parent root file at the top or bottom of the sub-document.

## The Non-Subtractive Principle (Knowledge Preservation)

When updating documentation, plans, or technical specifications:
- **Preserve Deep Knowledge**: Contributors and AI agents are strictly prohibited from silently deleting technical specifics, configuration values, port mappings, edge case explanations, or architectural rationales.
- **Carry-Forward Rule**: Before rewriting or replacing any documentation, scan for existing domain knowledge and ensure all operational details are preserved in the updated version.

## Frontmatter Synapses & Cross-Referencing

All documentation deep-dives across all 5 taxonomy areas—living specifications (`docs/specs/`), developer runbooks (`docs/development/`), architectural decision records (`docs/adr/`), topology deep-dives (`docs/architecture/`), and design specifications (`docs/design/`)—must include standard YAML frontmatter with tags and synapses (relative markdown links) to establish clear traceability:

```yaml
---
title: Feature Living Spec Title
status: active
tags: [auth, jwt, security]
synapses: ["docs/adr/0001-project-identity-jarn.md"]
---
```
