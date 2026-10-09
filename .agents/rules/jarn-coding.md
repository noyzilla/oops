---
trigger: always_on
description: "Coding style, visual hygiene, numbering invariants, and English-only agent rules."
---

# Coding & Documentation Style

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Add project-specific style rules to your project's `AGENTS.md` or `docs/development/` only.

This document defines the visual hygiene, bilingual annotation rules, and code commenting invariants for all project files.

## Visual Hygiene Invariant

Strictly **NO emojis or decorative icons** in production code, commit subjects, build logs, terminal output, or technical spec comments. All comments, log messages, and spec texts must maintain clean visual hygiene to prevent parser noise and noisy Git diffs.

## Documentation Language Standard

All **user-facing** governance documents, specs, and architectural guidelines must strictly follow the **Documentation Language Policy** defined in `AGENTS.md` (e.g., `en`, `th`, `en-th`).
- If `en` is active: Use English only.
- If a bilingual policy (e.g., `en-th`) is active: Use English as the lead technical language, followed by annotations in the secondary language enclosed in brackets `[...]` for human context scanning.
- If `th` is active: Use Thai as the lead language, but all technical terminology, variables, API paths, and code symbols MUST be written in English.

**Scope Limit**: This policy applies to root-level documents (`AGENTS.md`, `CONTEXT.md`, `README.md`, `ARCHITECTURE.md`, `DESIGN.md`) and `docs/` content that humans read directly. Machine-readable agent instructions inside `.agents/rules/` and `.agents/skills/` MUST ALWAYS use **English only** to minimize token consumption per session, regardless of the active language policy.

## Documentation & Commenting Rules

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
  - Operational Gates and Milestones (e.g., `GATE 1 — Living Spec & Approval`, `GATE 2 — Self-Verification`, `GATE 3 — Knowledge Capture`)
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
### GATE 1 — Living Spec & Mission Approval
### GATE 2 — Surgical Execution & Self-Verification
### GATE 3 — Knowledge Capture & Pre-Merge Audit
```

### Code Comments Invariants
- Never use sequential numbered comments in source code (such as `// 1. Validate user`, `// 2. Load account`). Inserting an intermediate step forces renumbering all subsequent comments and generates noisy Git diffs.
- Code comments should be intent-based and explain why something is done rather than merely describing what the code already does.
- Strictly NO emojis or decorative icons in technical documentation or codebase comments.
