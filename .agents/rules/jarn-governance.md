---
trigger: always_on
description: "Core governance, safety boundaries, escalation gates, and workflow state machine."
---

# Governance, Workflow, and Safety

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Extend governance behavior through your project's `AGENTS.md` and `REVIEW.md` only.

This document establishes the safety boundaries, escalation gates, and core operational lifecycle for all contributors (human engineers and AI agents) operating within this codebase.

## Safety Boundaries & Non-Negotiable Invariants

The following actions require prior explicit human confirmation:

- **Database Destruction**: Dropping databases, schemas, or tables, truncating tables, or applying unverified data-destructive migrations.
- **Git History Rewrite**: Force-pushing (`git push --force` or `--force-with-lease`), deleting remote branches, or hard-resetting shared branches.
- **Direct Edits & Commits to Main (Step 0 Invariant)**: Modifying or committing directly on `main`. Verify `git branch --show-current` and branch out (`git checkout -b <type>/<slug>`) before editing. **Exception**: `chore(release): vX.Y.Z` post-merge commits via `jarn-release`.
- **Credential Exposure**: Adding, modifying, reading, or printing production secrets, private keys, API keys, or `.env` files.
- **Uncontrolled Dependencies**: Adding new third-party libraries without architectural approval.
- **Unbounded Deletion**: Recursively deleting directories or bulk deleting files outside build or scratch folders.
- **Root Pollution**: Creating scratch files, payloads, scripts, or logs in project root. Place artifacts in `.scratch/<task-slug>/tmp/`.

## Stop and Ask Escalation Gates

Contributors and agents must pause execution and consult when any of the following conditions arise:

- **Ambiguous Requirements**: The request lacks clear acceptance criteria or presents multiple conflicting implementation paths.
- **Architectural Deviation**: An intended change conflicts with patterns established in `ARCHITECTURE.md` or active Living Specifications (`docs/specs/`).
- **Deep Blocking or Cycle Detected**: An issue chain in `.scratch/<task-slug>/issues/` exceeds depth 2 (Blocker's Blocker blocked) or forms a circular dependency, signaling architectural foundation failure.
- **Unforeseen Impact**: Modifying a module introduces cascading errors or breaks contracts across dependent modules.
- **Scope Expansion**: The implementation requires touching files or services beyond the boundaries of the approved plan.

## Consult First, Act Second

Every non-trivial modification follows a disciplined progression from inquiry to verified execution. Never make unilateral code modifications during planning phases.

### Inquiry vs Directive State Machine
- **Inquiry Mode (Default / Consultation)**: All conversational requests are treated as Inquiry Mode by default. The agent is strictly prohibited from mutating application source files, remaining in read-only analysis, design debate, or living spec drafting mode.
- **Defect Inquiry Invariant**: Reporting an issue, bug, or error log treats the conversation as Inquiry Mode. The agent MUST investigate and present the 4-step Diagnostic Report (Symptom, Root Cause, Impact/Blast Radius, Proposed Fix) before requesting an execution directive. Unilateral code edits without a directive are strictly prohibited.
- **Directive Mode (Explicit Execution Trigger)**: The agent transitions to Directive Mode only upon explicit human directive (e.g., "ทำเลย", "เริ่มแก้ได้", "อนุมัติ", "proceed", or approving an implementation plan). Without an explicit directive, the agent must continue consultation and refine specifications.
- **Directive Shortcuts (Aliases)**: To reduce friction, the following short commands are recognized as explicit directives (requiring a prefix to prevent accidental triggers):
  - `\p` or `!p`: Proceed (Approve plan and execute in the working tree).
  - `\ok` or `!ok`: Approve and proceed.
  - `\c` or `!c`: Commit (Approve the uncommitted diff and authorize `git commit`).
- **Plan Approval vs. Commit Authority**: Plan approval grants authority to modify files and run verification tools in the Working Tree only. It does NOT grant blanket commit authority. Commits require explicit sub-task review confirmation.
- **Inquiry Trade-offs**: When discussing architectural or non-trivial implementations, present at least two viable implementation options with technical trade-offs before requesting an execution directive.
- **Ambiguous Directive Fallback (Safety Brake)**: If a vague directive is given without established context or approved plan, fall back to Inquiry Mode and ask for clarification.

## Collaborative Spec Protocol & Change Taxonomy

To prevent misaligned implementations, unnecessary documentation churn, and AI context pollution, modifications are governed by a strict change taxonomy:

### The Change Taxonomy
- **Spec-Altering Changes (New Features & Behavioral Shifts)**:
  - Scope: Introduces new capabilities, alters business formulas, modifies API schemas, or changes state transitions.
  - Requirement: Mandatory co-evolution. The code change must be accompanied by updates to `docs/specs/<feature>.md` in the same commit or pull request. Record additions and changes in `CHANGELOG.md` under `### Added` or `### Changed`.
- **Spec-Conforming Bug Fixes (Defects & Implementation Errors)**:
  - Scope: The specification was already correct, but the implementation deviated from it (such as typing strict equality instead of greater-than-or-equal, missing null checks, or syntax defects).
  - Requirement: The specification in `docs/specs/` must not be churned or modified unnecessarily because it was already the correct source of truth. Instead, write a clear explanation of the defect, root cause, and fix into the Git commit message (`fix:`), which serves as the authoritative engineering logbook. Include a regression test and record in `CHANGELOG.md` under `### Fixed`.
- **Internal Refactoring & Performance Optimizations**:
  - Scope: Restructuring internal code or optimizing runtime performance without altering observable contracts, APIs, or business logic.
  - Requirement: Specification remains unchanged. Document intent in the Git commit message (`refactor:` or `perf:`).
  - **Two-Pass Migration Pattern**: When deprecating or replacing shared interfaces/APIs, execute in two steps: first introduce the new interface and migrate all callers; once verified, delete the legacy interface in a subsequent commit. Never delete legacy entrypoints while callers remain active.

### Interactive Design Debate
- Before generating implementation plans or code for spec-altering changes, human and AI discuss intent, constraints, domain definitions, and technical trade-offs.
- The AI challenges assumptions, clarifies edge cases, and seeks alignment on core business invariants.

### Living Spec Synthesis (`docs/specs/`) — Spec is Law
- Once consensus is reached, the AI synthesizes the agreement into the subsystem living specification under `docs/specs/<subsystem>.md` using `.agents/templates/docs/spec.md`.
- **Current System Truth Invariant**: Living specifications define current system truth (e.g. `docs/specs/authentication.md`), never in-flight feature requests or task deltas (e.g., `docs/specs/add-oauth.md` is forbidden). All code implementations must strictly conform to the spec.
- The specification defines current truth: business rules, state machines, API contracts, and explicit verification criteria. Historical evolution is recorded cleanly in `CHANGELOG.md` and Git commit logs.

### Dependency & Blast-Radius Scoping
- Every living specification must document its Dependency & Blast-Radius Matrix (upstream callers, downstream dependencies, and affected packages).
- When resolving bugs, refactoring, or extending existing code, AI agents inspect the living spec's blast-radius matrix to strictly isolate their investigation and edits, eliminating wasteful full-codebase scans.

## Operational Lifecycle Gates

The full operational lifecycle (GATE 1 → GATE 2 → GATE 3) including phase definitions, gate checklists, and the Definition of Done is defined in [jarn-lifecycle.md](jarn-lifecycle.md).
