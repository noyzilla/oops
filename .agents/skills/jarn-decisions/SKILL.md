---
name: jarn-decisions
description: >-
  Manage the lifecycle of Architectural Decision Records (ADRs) in docs/decisions/,
  evaluate trade-offs, and handle active vs .deprecated.md/.superseded.md file renaming
  for zero-token filtering.
---

# Jarn Architectural Decision Records (ADR) Workflow

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the procedural runbook for proposing, drafting, evaluating, and retiring Architectural Decision Records under `docs/decisions/`.

## Core Philosophy

Architectural Decision Records capture strategic, macro-level architectural choices (such as choosing a database engine, introducing an event-driven bus, or setting an authentication mechanism). Unlike feature living specifications which evolve constantly, decision records document point-in-time rationale, trade-offs, and consequences.

To protect AI agents from context window bloat and outdated assumptions, inactive decisions are suffixed with `.deprecated.md` or `.superseded.md`. This enables automated zero-token filtering during agent file discovery.

## When to Use This Skill

Activate this workflow when:
- Making cross-cutting, strategic technology or architecture choices.
- Introducing a new external infrastructure dependency or framework.
- Evaluating architectural alternatives and documenting technical trade-offs.
- Deprecating or replacing a previously accepted architectural decision.

Do NOT use this workflow when:
- Designing feature-specific domain logic, schemas, or APIs (use `jarn-spec` in `docs/specs/`).
- Documenting operational setups, onboarding, or runbooks (use `docs/development/`).
- Making routine refactoring choices or routine dependency updates.

---

## Operational Execution Runbook

### Phase: Architectural Threshold Check
Before drafting a decision record, verify that the decision meets the strategic threshold:
- Does this affect multiple subsystems or the fundamental system topology?
- Is this choice difficult or costly to reverse later?
- Were multiple viable technological alternatives evaluated?
- If the answer is no, document the choice directly inside the relevant feature living specification or commit logbook instead.

### Phase: Alternatives & Trade-offs Evaluation
Collaborate with the human lead to evaluate viable options:
- Formulate the primary decision drivers and constraints (e.g., latency, cost, developer ergonomics, security).
- Analyze at least two competing options against the selected approach.
- Enumerate positive consequences (benefits, capabilities gained).
- Enumerate negative consequences (technical debt, operational overhead, migration burden).

### Phase: Decision Proposal & Approval Checkpoint
- Determine the next sequential four-digit identifier (e.g., `0001`, `0002`) and prepare a proposed ADR in the conversation.
- Present Context, Problem Statement, Decision, Consequences, and Evaluated Alternatives to the human lead.
- Keep the proposed status as `Proposed` and wait for an explicit directive before creating or modifying an ADR file.

### Phase: Synthesis & Drafting
- After explicit approval, create `docs/decisions/<id>-<slug>.md` using [docs/decisions/0000-template.md](../../../docs/decisions/0000-template.md).
- Record the approved Context, Problem Statement, Decision, Consequences, and Evaluated Alternatives.
- Keep status marked as `Proposed` until human lead sign-off.

### Phase: Superseding & Deprecation Lifecycle
When a new decision replaces or invalidates an existing ADR:
- **Superseding**:
  - Locate the existing active ADR (e.g., `0002-mongodb.md`).
  - Rename the file using Git: `git mv docs/decisions/0002-mongodb.md docs/decisions/0002-mongodb.superseded.md`.
  - In `0002-mongodb.superseded.md`, update status to `Superseded by ADR-[XXXX]` and add a direct markdown link to the new ADR.
  - In the new ADR, add a direct markdown link referencing the superseded ADR in the context section.
- **Deprecation**:
  - If a decision is retired without a direct replacement, rename the file using Git: `git mv docs/decisions/<id>-<slug>.md docs/decisions/<id>-<slug>.deprecated.md`.
  - Update status to `Deprecated` and document the rationale for retirement.

### Phase: Verification & Zero-Token Filtering
- Verify that only active decisions use the clean `[ID]-[slug].md` naming convention.
- Ensure all inactive decisions use `.deprecated.md` or `.superseded.md`.
- Run `git diff --check` to ensure no whitespace defects or broken links.
- Commit the decision with conventional commit prefix `docs(decision): ...`.
