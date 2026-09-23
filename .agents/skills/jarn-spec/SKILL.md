---
name: jarn-spec
description: >-
  Guide agents and developers in authoring, debating, and maintaining vertical slice
  living specifications in docs/specs/ using the Code-Spec Parity Invariant and
  Dependency & Blast-Radius Matrix.
---

# Jarn Living Specification Workflow

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the procedural runbook for drafting, refining, and maintaining living specifications under `docs/specs/<feature>.md`.

## Core Philosophy

A living specification represents the current truth of an application subsystem. It captures business rules, data schemas, state transitions, and blast radiuses in a single vertical slice. Living specifications evolve in lockstep with the codebase (Code-Spec Parity Invariant).

## When to Use This Skill

Activate this workflow when:
- Designing a new capability, service, or domain subsystem.
- Altering business logic, calculation formulas, or state machines (Spec-Altering Change).
- Modifying API endpoints, RPC contracts, request schemas, or response schemas.
- Adding or modifying persistent database tables, columns, or relations.

Do NOT use this workflow when:
- Fixing an implementation bug where the existing spec is already correct (record the defect in the Git commit logbook).
- Performing internal code refactoring or styling cleanups that preserve external behavior.

---

## Operational Execution Runbook

### Phase: Classification & Discovery
- Verify the change classification against the Change Taxonomy in `.agents/rules/jarn-governance.md`.
- Ensure the task is Spec-Altering (introduces or changes observable behavior).
- Check `docs/specs/` to determine if a specification for this subsystem already exists.
- If it exists, identify it as the proposed update target. If new, identify `docs/specs/<feature-slug>.md` as the proposed file path using [docs/specs/0000-template.md](../../../docs/specs/0000-template.md).

### Phase: Interactive Design Debate
Before writing specification details or code, conduct an architectural debate with the human lead:
- Clarify intent, functional scope, and edge boundaries.
- Define domain terminology and align with [CONTEXT.md](../../../CONTEXT.md). Prohibit ambiguous or misleading synonyms.
- Challenge edge cases: null handling, boundary conditions, concurrency, idempotency, and network failures.
- Present at least two viable architectural or technical options with concrete trade-offs when significant uncertainty exists.

### Phase: Specification Synthesis
Draft the specification following the structure defined in [docs/specs/0000-template.md](../../../docs/specs/0000-template.md):
- **Overview & Scope**: Problem statement and subsystem boundaries.
- **Domain Context & Ubiquitous Language**: Canonical definitions and forbidden synonyms.
- **Business Rules & Logic Invariants**: Explicit calculation methods, validation constraints, and edge handling.
- **Flow & State Machine**: State transitions, trigger events, and failure recovery.
- **Interface & Data Contracts**: Endpoints, input/output schemas, error responses, and database impacts.
- **Dependency & Blast-Radius Matrix**: Bounded callers, dependencies, and files affected.
- **Verification & Acceptance Criteria**: Specific test commands, covered files, and concrete happy/error path scenarios.

### Phase: Specification Approval Checkpoint
- Present the proposed specification in the conversation, including the affected file path and implementation scope.
- Wait for explicit human approval before creating or modifying a specification file.

### Phase: Dependency & Blast-Radius Calculation
Define the blast radius with precision:
- Identify **Inbound Callers**: Which controllers, CLI commands, background workers, or external services invoke this spec?
- Identify **Outbound Dependencies**: Which third-party libraries, databases, caches, or sub-modules are called?
- Enumerate **Bounded Blast Radius**: List the exact file paths, directories, and test suites that require inspection and execution when this spec changes.

### Phase: Verification Definition
- Define the project-native verification command to validate this subsystem (e.g., package-specific test command).
- Ensure the acceptance criteria provide unambiguous assertions that can be validated via automated tests.

### Phase: Synchronized Implementation & Review
- After explicit human approval, create or update the specification on an isolated branch.
- Implement code changes on an isolated branch (`feat/...`) with incremental micro-commits.
- Maintain code-spec parity: commit the code changes and the updated `docs/specs/<feature-slug>.md` together.
- Record the addition or modification in `CHANGELOG.md` under `[Unreleased]`.
