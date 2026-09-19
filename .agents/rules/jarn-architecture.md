# Architecture & Lifecycle Standards [มาตรฐานสถาปัตยกรรมและวงจรชีวิต]

This document establishes the architectural principles, documentation topology, and native ecosystem lifecycles for the project.

## Ecosystem-Native Lifecycle Contract [สัญญาขั้นตอนปฏิบัติประจำระบบ]

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

### Global vs. Scoped Placement [การแยกส่วนกลางและส่วนเฉพาะ]
- **Rule:** System-wide or cross-module settings must reside at the root level (e.g., `PORT`, `PROXY_URL`, `DEBUG`). Domain-specific settings must be encapsulated within their respective domain objects (e.g., `DATABASE`, `CACHE`, `MAILER`).
- **Bad:** Hiding `PORT` inside `CONFIG.SERVER.PORT` when it is globally required.
- **Good:** `CONFIG.PORT` for global settings and `CONFIG.CACHE.TTL_SECONDS` for scoped settings.

### Strict Separation of Config vs. Internal Constants [แยกสิ่งที่ตั้งค่าได้ ออกจากค่าคงที่ภายในระบบ]
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
- Developer onboarding, runbooks, and operational workflows go into `docs/development/<runbook>.md` (mirroring `CONTRIBUTING.md`)
- Macro architectural decision records go into `docs/decisions/<id>-<slug>.md` (with explicit `.deprecated.md` or `.superseded.md` lifecycle naming for zero-token AI filtering)

### Two-Way Linking Requirement
Whenever a sub-document is created under `docs/<name>/`:
- Add a bullet point and link under the deep-dive index section of the root parent file.
- Add a backlink to the parent root file at the top or bottom of the sub-document.

## The Non-Subtractive Principle (Knowledge Preservation)

When updating documentation, plans, or technical specifications:
- **Preserve Deep Knowledge**: Contributors and AI agents are strictly prohibited from silently deleting technical specifics, configuration values, port mappings, edge case explanations, or architectural rationales.
- **Carry-Forward Rule**: Before rewriting or replacing any documentation, scan for existing domain knowledge and ensure all operational details are preserved in the updated version.

## Frontmatter Synapses & Cross-Referencing [โปรโตคอลการเชื่อมโยงข้อมูลแบบ Synapse]

All living specifications (`docs/specs/`) and architectural decision records (`docs/decisions/`) must include standard YAML frontmatter with tags and synapses (relative markdown links) to establish clear traceability:

```yaml
---
title: Feature Living Spec Title
status: active
tags: [auth, jwt, security]
synapses: ["docs/decisions/0001-project-identity-jarn.md"]
---
```
