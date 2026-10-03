---
trigger: always_on
description: "Operational lifecycle gates (GATE 1 to GATE 3) and Definition of Done."
---

# Operational Lifecycle Gates

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Extend project-specific gates through your project's `REVIEW.md` only.

This document defines the three sequential operational gates every contributor and AI agent must follow from requirement discovery and specification through knowledge capture. See [jarn-governance.md](jarn-governance.md) for safety boundaries and workflow rules.

## Lifecycle Overview

```
GATE 1 → GATE 2 → GATE 3
```

- **GATE 1: Living Spec & Mission Approval**: Requirement discovery, brainstorming, and living spec synthesis. The AI leads inquiry and brainstorms options (`jarn-consult`), acts as a Socratic coach, and synthesizes living specs (`jarn-spec`) and implementation plans. Human green light required before touching application code.
- **GATE 2: Surgical Execution & Self-Verification**: Branch isolation, micro-commits, targeted verification, test suite Exit Code 0.
- **GATE 3: Knowledge Capture & Pre-Merge Audit**: Code-spec parity, TASK.md synchronization, universal quality gate checklist audit (`jarn-review`).

---

### GATE 1: Living Spec & Mission Approval (The Specification Gate)

- **Consultation & Brainstorming Protocol**:
  - Activate `jarn-consult` skill before every spec-altering change (new features, behavioral shifts, API contract changes).
  - Skip consultation only for spec-conforming bug fixes, internal refactors, or emergency production hotfixes where the spec is already correct and scope is unambiguous.
  - The AI classifies the request, probes intent with focused questions, challenges assumptions, and proposes 2–3 implementation options with concrete trade-offs before spec synthesis.
  - Evaluates viability early: determines whether the request should proceed, be deferred, or marked "won't do" (YAGNI filter).
- **Anti-Hallucination Discovery & Research**:
  - Empirically verify library versions, external APIs, and project configurations via terminal commands or official docs before proposing solutions.
- **Living Spec & Transient Plan Synthesis**:
  - Produce or update the living specification in `docs/specs/<feature>.md` using `docs/specs/0000-template.md` (`jarn-spec`).
  - Author the branch-scoped execution plan in `.scratch/<task-slug>/plan.md` and decompose into discrete issue files under `.scratch/<task-slug>/issues/XXXX-<slug>.md`.
- **Hard Stop**: Halt execution and wait for explicit human green light (Directive Mode) before touching application code.
- GATE 1 operates entirely within Inquiry Mode. No codebase mutations occur during this gate.

---

### GATE 2: Surgical Execution & Self-Verification (The Dev Pairing Gate)

- **Step 0 Branch Isolation**: Before modifying, creating, or deleting any codebase file, verify `git branch --show-current`. If on `main`, immediately execute `git checkout -b <type>/<slug>`. Working directly on `main` is strictly prohibited.
- **Working Tree Loop by Default**: All changes across the codebase remain in the working tree uncommitted by default. Approving an implementation plan authorizes coding and self-verification in the working tree only; it does NOT grant blanket commit authority.
- **Discrete Issue Execution**: Work through issue files in `.scratch/<task-slug>/issues/` in sequence. Upon completion and passing verification, micro-commit and transition the issue file to `XXXX-<slug>.done.md`.
- **Never Derail Workflow**: Mid-flight discoveries or missing sub-tasks must be appended as new issue files in `.scratch/<task-slug>/issues/` rather than derailing the active work.
- **Scratch Sandbox Isolation**: Temporary reproduction scripts, mock payloads, or diagnostic logs must reside strictly in `.scratch/<task-slug>/tmp/`. Writing scratch files to the project root is strictly prohibited.
- **Sub-task Micro-Commit Sequencing**: The Driver agent implements the active issue in the working tree, runs targeted verification, and presents the uncommitted diff for Navigator review. Execute `git commit` only upon explicit confirmation.
- **Extended Verification on Demand**: Defect fixes occur immediately in the working tree with zero undo-commit overhead.
- **No Fixup Noise (Amend Invariant)**: Amend or soft-reset (`git reset --soft HEAD~1`) on recently completed commits within the active branch rather than stacking fragmented fixup commits.
- **Targeted Verification**: Check `git status` before running verification commands. Execute project-native test suites and linters via terminal to verify Exit Code 0 and zero regression.

---

### GATE 3: Knowledge Capture & Pre-Merge Audit (The Senior / Lead Review Gate)

- **Senior / Lead Authority**: Macro-level system inspection conducted by the Senior Lead before integrating changes into `main`.
- **System-wide Integrity & Security**: Verify that modified modules do not cause downstream regressions, secret leaks, or contract breakages across the entire application.
- **Code-Spec Parity Verification**: Ensure code implementations match living specs in `docs/specs/<subsystem>.md`.
- **Evidence Attachment**: Attach empirical test execution logs demonstrating clean passing results (Exit Code 0).
- **Pre-Merge Audit Execution**: Activate and fulfill the universal quality gate checklist in [jarn-quality.md](jarn-quality.md) and project extensions in `REVIEW.md` via the `jarn-review` skill.
- **Knowledge Capture & Zero Git Clutter**: When all issues in `.scratch/<task-slug>/issues/` are `.done.md`, co-evolve living specs in `docs/specs/`, record changes in `CHANGELOG.md` under `[Unreleased]`, and update milestones in root `TASK.md`. Because `.scratch/` is git-ignored, it leaves zero git trace on `main`.
- **Continuous Flow (Default)**: Complete GATE 3, audit against `REVIEW.md`, and execute the merge autonomously once authorized by the Senior/Lead role.
- **Handoff Interruption (Brake Flow)**: Halt execution and perform a handoff if `CONTRIBUTING.md` mandates a role handoff (e.g., QA) or if the human lead instructs. Update `plan.md`, commit with `handoff(<target>): <message>`, push to origin, and halt.
- Provide a concise walkthrough of changes and test results, then conclude the task cleanly.

---

## Definition of Done (DoD)

A task is considered complete only when all the following criteria are satisfied:

- **Evidence-Based Completion**: Concrete empirical evidence (passing test suite output, compiler logs, or behavioral confirmation) must be produced. A task is never marked done based solely on code generation without verification.
- **Blast-Radius Verification**: Verification commands strictly matched the modified surfaces without executing unrelated checkers.
- All unit and integration tests pass without failure.
- Static analysis and code formatting checks pass cleanly.
- New or modified logic includes adequate test coverage.
- Related documentation (`ARCHITECTURE.md`, `DESIGN.md`, or `docs/`) is synchronized.
- Git commit messages comply with project commit conventions.
