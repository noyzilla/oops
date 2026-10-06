---
name: jarn-diagnostics
description: >-
  This skill performs isolated, blast-radius-scoped defect diagnostics. It locates the feature spec
  in docs/specs/, consults the dependency matrix, writes reproducing tests, and
  applies approved surgical fixes without blind full-codebase scans.
---

# Jarn Defect Diagnostics & Blast-Radius Scoping

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the surgical diagnostic runbook for investigating and resolving bugs without causing context pollution or uncoordinated full-codebase scans.

## Core Philosophy

When an incident or defect occurs, naive AI agents often execute costly full-codebase searches or speculate broadly. This skill anchors diagnostics in the subsystem's living specification and its **Dependency & Blast-Radius Matrix**, isolating investigation to the bounded architectural surface.

## When to Use This Skill

Activate this workflow when:
- Investigating runtime errors, test failures, or defect reports.
- Diagnosing regressions following dependency upgrades or merges.
- Formulating a reproducing test case for reported bugs.
- Applying a spec-conforming surgical fix.

Do NOT use this workflow when:
- Adding a brand-new feature or expanding business contracts (use `jarn-spec`).
- Conducting general exploratory codebase tours or onboarding.

---

## Operational Execution Runbook

### Phase: Subsystem & Spec Localization
- Inspect the error stack trace, log output, or symptom description to identify the failing component.
- Locate the authoritative living specification under `docs/specs/<subsystem>.md`.
- Read the specification to understand expected invariants, state transitions, and business formulas.
- Verify whether the issue is a **Spec-Conforming Bug** (implementation broke existing spec) or a **Spec-Altering Defect** (spec was underspecified or wrong).

### Phase: Blast-Radius Matrix Extraction
Inspect the `## Dependency & Blast-Radius Matrix` section inside `docs/specs/<subsystem>.md`:
- Note the **Inbound Callers** (who triggers this subsystem).
- Note the **Outbound Dependencies** (external calls, databases, downstream services).
- Note the **Bounded Blast Radius** (the specific files and packages eligible for inspection).
- **Rule of Isolation**: Restrict file reading, grep searches, and edits strictly to the bounded files listed in the matrix. Do NOT scan unrelated repository folders.

### Phase: Reproduction & Root Cause Evidence
- Locate an existing automated test or define a reproducing test plan within the bounded test suite identified in the living specification.
- **Diagnostic First Invariant**: Defect reports and error discussions are strictly Inquiry Mode. The AI MUST investigate and report findings without mutating codebase files.
- If an existing reproducer is available, execute the targeted test command to confirm the expected failure and retain its log as empirical evidence.
- Trace the divergence between the failing code and the specification invariants.

### Phase: Diagnosis Report & Approval Checkpoint
Present a structured 4-step Diagnostic Report before requesting execution authorization:
1. **Symptom & Discovery**: Observed behavior, error tracebacks, or test failures.
2. **Root Cause Analysis**: Underlying cause (why it broke, spec divergence, or unhandled edge case).
3. **Blast Radius & Impact**: Bounded files, upstream callers, and downstream dependencies affected.
4. **Proposed Fix & Verification Plan**: Minimal surgical repair plan and reproducing test strategy.

- **Hard Stop**: Stop after presenting the 4-step report. Wait for an explicit human directive ("ทำเลย", "อนุมัติ", "proceed") before writing tests or applying surgical code fixes.

### Phase: Root Cause Analysis & Surgical Patching
- Add or update the approved reproducing test and confirm it fails for the expected reason (Red state).
- Determine the minimal surgical fix required to satisfy the invariant.
- Apply the fix strictly within the bounded source files.
- Preserve all surrounding code formatting, existing comments, and adjacent behavior.

### Phase: Bounded Regression Verification
- Run the targeted test command to verify that the reproducing test now passes cleanly (Green state).
- Run the subsystem's existing regression suite to confirm that no adjacent behaviors were broken.
- Verify that `git status` shows only the intended bounded files modified.

### Phase: Git Commit Logbook Entry
- Comply with the Spec-Conforming Bug Fix standard:
  - Do NOT modify `docs/specs/<subsystem>.md` if the spec was already correct.
  - Structure the Git commit message as an authoritative engineering logbook entry (`fix:`):
    - Describe the defect symptom.
    - Detail the empirical root cause.
    - Document the surgical fix applied.
    - Reference the reproducing test added.
- Record the fix in `CHANGELOG.md` under `### Fixed`.

---

## Diagnostics Checklist

Copy this checklist into the response and tick each item as it is satisfied.

- [ ] Living spec located and bug classified (Spec-Conforming or Spec-Altering). If Spec-Altering, stop and hand off to `jarn-spec`.
- [ ] Blast-radius matrix read and all inspection limited to its bounded files. If the root cause lies outside the matrix, return to Phase: Blast-Radius Matrix Extraction and flag the spec as incomplete.
- [ ] Reproducer fails for the expected reason (Red). If it fails for another reason, return to Phase: Reproduction & Root Cause Evidence.
- [ ] Explicit directive received before any test or fix is written.
- [ ] Fix applied and reproducer passes (Green). If the regression suite fails, return to Phase: Root Cause Analysis & Surgical Patching and revise the fix.
- [ ] `git status` shows only intended bounded files; commit logbook and `CHANGELOG.md` entry written.
