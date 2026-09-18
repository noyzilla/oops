# Engineering Standards & Conventions [มาตรฐานวิศวกรรมและข้อตกลง]

This document defines the unified engineering, commit, documentation, and code style standards enforced across the repository.

## Visual Hygiene Invariant [มาตรฐานความสะอาดทางสายตา]

Strictly **NO emojis or decorative icons** in production code, commit subjects, build logs, terminal output, or technical spec comments. All comments, log messages, and spec texts must maintain clean visual hygiene to prevent parser noise and noisy Git diffs.

## Bilingual Documentation Annotation Standard [มาตรฐานการเขียนแบบสองภาษา]

All core governance documents, specs, and architectural guidelines must use **English as the lead technical language** for precision and AI scanning compatibility, followed by **Thai annotations in brackets `[...]`** for human developer intuition and rapid context scanning.

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

## Commit Conventions [ข้อตกลงการคัดส่งโค้ด]

This project strictly follows the Conventional Commits specification coupled with Semantic Versioning (SemVer).

### Format
```text
<type>(<scope>): <subject>
```

### Subject Rules
- **Imperative mood**: Use verbs like `add`, `fix`, `refactor` rather than `added` or `fixes`.
- **Lowercase**: Start the subject with a lowercase letter.
- **No trailing period**: Do not place a period at the end of the subject line.

### Commit Types & SemVer Impact

| Prefix | Description | SemVer Bump | Example |
| :--- | :--- | :---: | :--- |
| **`feat:`** | Introduces a new feature | **MINOR** (`v0.1.0` -> `v0.2.0`) | `feat(auth): add jwt validation middleware` |
| **`fix:`** | Fixes a bug | **PATCH** (`v0.1.0` -> `v0.1.1`) | `fix(db): handle connection timeout gracefully` |
| **`feat!:`** or `BREAKING CHANGE:` | Introduces breaking changes | **MAJOR** (`v0.1.0` -> `v1.0.0`) | `feat!(api): require token header on all routes` |
| **`docs:`** | Documentation changes only | None / Patch | `docs: update deployment architecture guide` |
| **`refactor:`** | Code change that neither fixes a bug nor adds a feature | None / Patch | `refactor: simplify target validation` |
| **`test:`** | Adding or updating tests | None | `test: add unit tests for token parser` |
| **`chore:`** | Tooling, build scripts, or maintenance | None | `chore: update build script dependencies` |

## Commit Frequency & Granularity (Micro-Commit Strategy) [กลยุทธ์การคัดส่งแบบย่อย]

To ensure code stability, bisectability, and rapid troubleshooting, all contributors and agents must follow an incremental micro-commit workflow:

- **Atomic Milestones**: Commit code incrementally as each cohesive sub-task is completed and verified. Do not accumulate large, uncommitted diffs across multiple components.
- **Granular Checkpointing**: Save commits after completing logical units of work (such as introducing a failing test, implementing a specific helper, or refining documentation). This creates safe rollback points and allows comparing behavioral changes across intermediate stages.
- **Clean Context Switching**: Working in isolated branches with frequent commits ensures an engineer or agent can switch contexts to address an urgent production hotfix immediately without losing in-flight feature work.
- **No Big-Bang Commits**: Committing an entire days-long or multi-component task in a single monolithic commit at the end of development is strictly prohibited.

## Documentation & Commenting Rules [กฎระเบียบเอกสารและการบันทึกโค้ด]

All code comments, markdown files, specifications, plans, walkthroughs, and architecture records must prevent formatting maintenance overhead and maintain resilient, location-agnostic references.

### Stable References (Avoid Positional Coupling)
Never create references based on relative position, sequence numbers, or hardcoded set counts:
- Avoid positional references such as "Step 3", "the second item", "the section above", "the previous section", or hardcoded collection counts like "the five directories".
- Prefer stable, descriptive references and direct links: `Authentication`, `Transaction Processing`, `Retry Policy`, or `[Transaction Processing](docs/specs/0001-tx.md)`.
- Positional references break and corrupt documentation whenever items are inserted, removed, or reordered; semantic references remain permanently resilient.

### Numbering Invariant: Positional Order vs. Semantic Identity
Core principle: **Numbers used merely for sequence or reading order are prohibited; numbers that form an integral part of semantic meaning are permitted and required.**

- **Do NOT use numbered steps merely to indicate reading or execution order**: Never use `1.`, `2.`, `3.`, or `### 1.` in documentation, markdown files, plans, walkthroughs, or code comments solely to indicate sequence.
- **Prefer descriptive headings and unnumbered bullet points**: Readers naturally understand the intended order from top to bottom. Use unnumbered bullets (`-`) for workflows, procedures, collections, categories, concepts, and checklists.
- **Use numbers only when the number itself has semantic meaning**: Numbers are permitted and expected when they represent domain concepts, semantic milestones, or specific identifiers, including:
  - Operational Gates and Milestones (e.g., `GATE 0 — Mission Approval`, `GATE 1 — Self-Verification`, `GATE 2 — Knowledge Capture`)
  - Phases and milestones (e.g., `Phase 1 — Schema Migration`, `Phase 2 — Data Migration`)
  - Versions and protocol standards (e.g., `OAuth 2.0`, `TLS 1.3`, `HTTP 200`, `IPv6`)
  - Retries, backoffs, and priorities (e.g., `Retry 1 (immediate)`, `Retry 2 (30s backoff)`, `P1 Critical`)
  - Architectural tiers and cache levels (e.g., `L1 Cache`, `Tier 1 Support`)
  - Numbered formal requirements (e.g., `REQ-101`, `RFC 2119`)

#### Illustrative Contrast: Execution Order vs. Semantic Identity

Execution or reading order (No numbers needed; top-to-bottom list communicates sequence naturally without diff churn upon step insertion):
```markdown
## Deployment
- Build application
- Deploy container
- Run migration
- Verify deployment
```

Semantic Identity (Numbers represent named domain phases referenced across tickets, plans, and releases):
```markdown
## Operational Gates
### GATE 0 — Mission Approval
### GATE 1 — Self-Verification
### GATE 2 — Knowledge Capture
```

### Code Comments Invariants
- Never use sequential numbered comments in source code (such as `// 1. Validate user`, `// 2. Load account`). Inserting an intermediate step forces renumbering all subsequent comments and generates noisy Git diffs.
- Code comments should be intent-based and explain why something is done rather than merely describing what the code already does.
- Strictly NO emojis or decorative icons in technical documentation or codebase comments.

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
