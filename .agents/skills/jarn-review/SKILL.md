---
name: jarn-review
description: >-
  Automate pre-merge review and quality gate verification (GATE 3). Inspects git status, maps
  files to the Change Routing Matrix, executes targeted tests, verifies code-spec
  parity, and audits compliance against jarn-quality.md.
---

# Jarn Pre-Merge Review & Verification Runbook

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the autonomous quality gate review procedure executed in GATE 3 before opening, approving, or merging pull requests.

## Core Philosophy

Quality assurance in an AI-assisted environment must be deterministic, evidence-based, and targeted. This skill enforces the Universal Quality Gates & Pre-Merge Standard ([.agents/rules/jarn-quality.md](../../rules/jarn-quality.md)) and project-specific review extensions in `REVIEW.md` without running wasteful, cargo-cult test executions across untouched tech stacks.

## When to Use This Skill

Activate this workflow when:
- Concluding a feature branch, bug fix, or refactoring task in GATE 3.
- Preparing a pull request or requesting final human sign-off.
- Conducting a peer review or automated audit of proposed changes.
- Verifying Definition of Done (DoD) compliance.

Do NOT use this workflow when:
- Actively iterating on incomplete scratch code mid-implementation.
- Exploring codebase structure during early discovery phases.

---

## Operational Execution Runbook

### Phase 1: Surface Inspection & Safety Audit
- Run `git status` to identify all staged, unstaged, and untracked files.
- Run `git diff --stat` against the base branch (usually `main`) to view modified file paths and changed surface area.
- Verify that changes are contained within a dedicated feature branch (`feat/...`, `fix/...`, `docs/...`). Flag immediately if working directly on `main`.
- Verify **Inquiry vs Directive Traceability**: Confirm that execution was authorized by an explicit directive trigger.

### Phase 2: Targeted Verification & Code Quality Audit
- Map each modified file extension and path against the Change Routing Matrix in [AGENTS.md](../../../AGENTS.md):
  - Shell scripts (`*.sh`): Targeted check via `sh -n <file>`.
  - Application logic: Targeted unit tests for the affected package or module.
  - API endpoints: Integration/contract tests for the affected route.
  - Documentation/Markdown: Markdown formatting and link checks via `git diff --check`.
  - Living Specs (`docs/specs/`): Verification against interface contracts and blast radius.
- **Targeted Verification Invariant**: Do NOT execute linters, compilers, or test suites for programming languages or modules that were not modified in the current changeset.
- Execute strictly the targeted commands and capture stdout/stderr logs as empirical verification evidence proving Exit Code 0.
- **Visual Hygiene & Code Audit**: Ensure strictly zero emojis/icons in code, logs, and comments; verify no sequential numbered comments (`// 1.`, `// 2.`).

### Phase 3: Knowledge Capture, Parity & Evidence Synthesis
- **Code-Spec Parity Audit**:
  - If Spec-Altering Change: Verify corresponding `docs/specs/<feature>.md` was updated in lockstep with frontmatter synapses/tags.
  - If Spec-Conforming Bug Fix: Verify `docs/specs/` was NOT unnecessarily churned; confirm regression test added and root cause detailed in commit message (`fix:`).
- **Engineering Standards Audit**: Verify Conventional Commits, micro-commit granularity, stable references, and semantic numbering.
- **Evidence Synthesis Report**: Compile an evidence-based summary detailing modified surfaces, targeted verification command logs (Exit Code 0), and DoD compliance against `jarn-quality.md` and `REVIEW.md`.
- Present the walkthrough report to the human lead for final merge authorization.
