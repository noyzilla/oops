# Universal Quality Gates & Pre-Merge Standard

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). To add project-specific checks, use your project's `REVIEW.md` under "Project-Specific Review Extensions" only.

This document establishes the **universal, project-agnostic** baseline review checklist and quality gates for all contributors and AI agents before merging pull requests. It applies to every project that adopts the Jarn framework.

After completing all checks below, continue with your project's `REVIEW.md` for project-specific gates.

## Review Protocol

- Review every pull request against all applicable sections below.
- Verify every matching row before approving or merging.
- Any deliberate deviation or deferred check must be explicitly justified in the pull request description.

---

## GATE 1: Safety, Consultation & Spec Approval Invariants

- [ ] **Branch Isolation (Step 0)**: Changes were developed on an isolated branch (`feat/...`, `fix/...`, `docs/...`, `chore/...`). Direct commits to `main` are strictly prohibited.
- [ ] **Inquiry vs Directive Traceability (GATE 1)**: Execution was preceded by consultation/brainstorming (`jarn-consult`), spec alignment (`jarn-spec`), and unlocked by an explicit human directive, preventing unverified code mutations.
- [ ] **No Destructive Operations**: No database truncation, table dropping, bucket deletion, or unverified irreversible migrations without explicit lead sign-off.
- [ ] **No Secret Exposure**: Zero credentials, API keys, private tokens, or unencrypted `.env` files committed to repository history.
- [ ] **Dependency Scrutiny**: No new external dependencies introduced without prior architectural agreement and documentation.

---

## GATE 2: Code Quality, Cleanliness & Self-Verification

- [ ] **Domain Alignment**: All new or modified entity names, database columns, and API parameters match the ubiquitous language defined in `CONTEXT.md`.
- [ ] **Intent-Based Naming**: Variables and functions reflect domain intent, not mechanism. Generic placeholders (`data`, `temp`, `helper`, `manager`, `process`) are avoided.
- [ ] **Boolean Predicates**: Boolean variables and functions use clear prefixes (`is_active`, `has_access`, `can_modify`, `should_retry`).
- [ ] **Clean Commentary & Visual Hygiene**: Code comments explain non-obvious rationale, constraints, or invariants rather than narrating syntax. No sequential numbered comments (`// 1.`, `// 2.`) and strictly zero emojis/icons.
- [ ] **No Dead Code**: No commented-out code, temporary debugging statements, or orphaned helper functions left behind.
- [ ] **Companion Unit Tests**: Every creation or modification of domain logic, services, or APIs includes corresponding test cases in the same changeset.
- [ ] **Evidence-Based Proof**: Concrete test execution logs or compiler outputs showing clean passing results (Exit Code 0) are attached.
- [ ] **Targeted Verification (No Blind Runs)**: Verification commands strictly match the blast radius of changes (checked via `git status`). No execution of unrelated syntax checkers, language linters, or test suites for untouched file extensions.
- [ ] **Full Test Suite Pass**: All relevant unit, integration, and contract tests pass with Exit Code 0.

---

## GATE 3: Documentation, Knowledge Preservation & Pre-Merge Sync

- [ ] **Change Taxonomy & Code-Spec Parity**:
  - For Spec-Altering changes: Living specifications in `docs/specs/` are updated in lockstep with code and include frontmatter synapses/tags.
  - For Spec-Conforming bug fixes: `docs/specs/` is not modified unnecessarily; the defect root cause and fix are detailed in the Git commit logbook with regression tests.
- [ ] **The Non-Subtractive Principle**: Existing technical specifics, configuration values, port mappings, and architectural rationales have been preserved.
- [ ] **The Mirror Index Pattern**: Root documentation files (`ARCHITECTURE.md`, `DESIGN.md`, `CONTRIBUTING.md`) remain lean summaries under ~200 lines; deep specs reside in `docs/` with two-way links.
- [ ] **ADR Status Lifecycle**: When modifying or superseding architectural decisions, updated the filename with `.deprecated.md` or `.superseded.md` to enable zero-token AI filtering.
- [ ] **Bilingual Annotation Standard**: User-facing documents use English lead technical language with Thai annotations `[...]` for human clarity. Files inside `.agents/` use English only — verified with `grep -rn '^#.*[ก-๙]' .agents/` returning zero matches.
- [ ] **Semantic Numbering & Stable References**: Verified that numbers are not used merely for reading or execution order; numbers are used only when they possess semantic identity (phases, versions, retries, priorities, gates); all cross-references avoid positional coupling and use semantic headings or direct links.
- [ ] **Changelog Synchronized**: User-facing changes are recorded under the `[Unreleased]` section of `CHANGELOG.md`.
- [ ] **Task State Synchronized**: `TASK.md` has been updated to reflect newly completed milestones and immediate next actions.
- [ ] **Task State Quality**: Backlog items and next actions in `TASK.md` include specific deliverable scope (affected files, modules, or contracts). No vague placeholders.

---

## Contributor & AI Accountability

- [ ] **Full Ownership**: The submitter understands every line of code in the PR and can explain all technical decisions independently.
- [ ] **No AI Slop**: The PR contains no speculative scaffolding, unverified code, or unresolved repetitive AI fix loops.
- [ ] **Conventional Commits & Logbook**: Commit history follows Conventional Commits with intent-based explanations acting as an authoritative engineering logbook.
