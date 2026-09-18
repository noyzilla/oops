---
name: jarn-review
description: >-
  Automate pre-merge review and quality gate verification. Inspects git status, maps
  files to the Change Routing Matrix, executes targeted tests, verifies code-spec
  parity, and checks commit conventions.
---

# Jarn Pre-Merge Review & Verification Runbook [คู่มือการตรวจรับรองงานก่อนรวม]

This skill defines the autonomous quality gate review procedure executed before opening, approving, or merging pull requests.

## Core Philosophy [ปรัชญาหลัก]

Quality assurance in an AI-assisted environment must be deterministic, evidence-based, and targeted. This skill enforces the Universal Pre-Merge Standard ([.agents/rules/jarn-review.md](../../rules/jarn-review.md)) across three operational gates without running wasteful, cargo-cult test executions across untouched tech stacks.

## When to Use This Skill [เมื่อใดควรใช้สกิลนี้]

Activate this workflow when:
- Concluding a feature branch, bug fix, or refactoring task.
- Preparing a pull request or requesting final human sign-off.
- Conducting a peer review or automated audit of proposed changes.
- Verifying Definition of Done (DoD) compliance.

Do NOT use this workflow when:
- Actively iterating on incomplete scratch code mid-implementation.
- Exploring codebase structure during early discovery phases.

---

## Operational Execution Runbook [ขั้นตอนการปฏิบัติงาน]

### GATE 0: Surface Inspection & Safety Audit [จุดตรวจความปลอดภัยและขอบเขตงาน]
- Run `git status` to identify all staged, unstaged, and untracked files.
- Run `git diff --stat` against the base branch (usually `main`) to view modified file paths and changed surface area.
- Verify that changes are contained within a dedicated feature branch (`feat/...`, `fix/...`, `docs/...`). Flag immediately if working directly on `main`.
- Verify **Inquiry vs Directive Traceability**: Confirm that execution was authorized by an explicit directive trigger.

### GATE 1: Targeted Verification & Code Quality Audit [จุดตรวจการสอบทานและคุณภาพโค้ด]
- Map each modified file extension and path against the Change Routing Matrix in [AGENTS.md](../../../AGENTS.md):
  - Shell scripts (`*.sh`): Targeted check via `sh -n <file>`.
  - Application logic: Targeted unit tests for the affected package or module.
  - API endpoints: Integration/contract tests for the affected route.
  - Documentation/Markdown: Markdown formatting and link checks via `git diff --check`.
  - Living Specs (`docs/specs/`): Verification against interface contracts and blast radius.
- **Targeted Verification Invariant**: Do NOT execute linters, compilers, or test suites for programming languages or modules that were not modified in the current changeset.
- Execute strictly the targeted commands and capture stdout/stderr logs as empirical verification evidence proving Exit Code 0.
- **Visual Hygiene & Code Audit**: Ensure strictly zero emojis/icons in code, logs, and comments; verify no sequential numbered comments (`// 1.`, `// 2.`).

### GATE 2: Knowledge Capture, Parity & Evidence Synthesis [จุดตรวจสเปกมีชีวิตและหลักฐาน]
- **Code-Spec Parity Audit**:
  - If Spec-Altering Change: Verify corresponding `docs/specs/<feature>.md` was updated in lockstep with frontmatter synapses/tags.
  - If Spec-Conforming Bug Fix: Verify `docs/specs/` was NOT unnecessarily churned; confirm regression test added and root cause detailed in commit message (`fix:`).
- **Engineering Standards Audit**: Verify Conventional Commits, micro-commit granularity, stable references, and semantic numbering.
- **Evidence Synthesis Report**: Compile an evidence-based summary detailing modified surfaces, targeted verification command logs (Exit Code 0), and DoD compliance.
- Present the walkthrough report to the human lead for final merge authorization.
