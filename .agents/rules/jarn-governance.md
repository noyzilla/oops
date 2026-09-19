# Governance, Workflow, and Safety [การกำกับดูแล ขั้นตอนการทำงาน และความปลอดภัย]

This document establishes the safety boundaries, escalation gates, and core operational lifecycle for all contributors (human engineers and AI agents) operating within this codebase.

## Safety Boundaries & Non-Negotiable Invariants

The following actions are strictly prohibited without prior explicit human confirmation:

- **Database Destruction**: Dropping databases, schemas, or tables, executing table truncation, or applying unverified data-destructive migrations.
- **Git History Rewrite**: Force-pushing (`git push --force` or `--force-with-lease`) to remote branches, deleting remote branches, or hard-resetting shared branches.
- **Direct Edits & Commits to Main (Step 0 Invariant)**: Modifying, creating, or committing files directly on the `main` or production branch. Before making any codebase changes, contributors and agents MUST verify `git branch --show-current` and branch out (`git checkout -b <type>/<slug>`).
- **Credential Exposure**: Adding, modifying, reading, or printing production secrets, private keys, authentication tokens, API keys, or `.env` files containing sensitive credentials.
- **Uncontrolled Dependencies**: Introducing new third-party libraries, packages, or external dependencies that have not been explicitly discussed and agreed upon.
- **Unbounded Deletion**: Recursively deleting directories or bulk deleting source files outside of designated build output or scratch folders.

## Stop and Ask Escalation Gates

Contributors and agents must pause execution and consult when any of the following conditions arise:

- **Ambiguous Requirements**: The request lacks clear acceptance criteria or presents multiple conflicting implementation paths.
- **Architectural Deviation**: An intended change conflicts with patterns established in `ARCHITECTURE.md` or active Living Specifications (`docs/specs/`).
- **Unforeseen Impact**: Modifying a module introduces cascading errors or breaks contracts across dependent modules.
- **Scope Expansion**: The implementation requires touching files or services beyond the boundaries of the approved plan.

## Consult First, Act Second [การปรึกษานำหน้าการลงมือทำ]

Every non-trivial modification follows a disciplined progression from inquiry to verified execution. Never make unilateral code modifications during planning phases.

### Inquiry vs Directive State Machine [ระบบสเตตัสสอบถามและการสั่งการ]
Human collaborators interact naturally and conversationally without being burdened to format long, rigid prompt syntaxes. The system enforces safety through an internal AI state machine:
- **Inquiry Mode (Default State / Consultation)**: All conversational requests, questions, or ideas are treated as Inquiry Mode by default. The agent is strictly prohibited from executing code-altering tools on application source files. The agent remains in read-only analysis, design debate, or living spec drafting mode.
- **Directive Mode (Explicit Execution Trigger)**: The agent may only transition to Directive Mode when the human lead provides an explicit execution directive (such as "ทำเลย", "เริ่มแก้ได้", "อนุมัติ", "proceed", or approving an implementation plan). Without an explicit directive, the agent must continue the consultation and refine specifications.
- **Inquiry Trade-offs**: When discussing architectural or non-trivial implementations, the agent must present at least two viable implementation options with technical trade-offs before requesting an execution directive.
- **Ambiguous Directive Fallback (Safety Brake)**: If the human lead provides a vague execution directive (e.g., "fix it", "แก้เลย") without a clearly established context, specific file scope, or prior approved plan, the agent must treat the directive as a potential high-blast-radius risk. The agent MUST fall back to Inquiry Mode and ask for clarification or propose a specific scoped plan before proceeding.

## Collaborative Spec Protocol & Change Taxonomy [ข้อตกลงสเปกและจำแนกประเภทการเปลี่ยนผ่าน]

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

### Interactive Design Debate
- Before generating implementation plans or code for spec-altering changes, human and AI discuss intent, constraints, domain definitions, and technical trade-offs.
- The AI challenges assumptions, clarifies edge cases, and seeks alignment on core business invariants.

### Living Spec Synthesis (`docs/specs/`)
- Once consensus is reached, the AI synthesizes the agreement into a living specification under `docs/specs/<feature-name>.md` using `docs/specs/0000-template.md`.
- The specification defines current truth: business rules, state machines, API contracts, and explicit verification criteria.
- Unlike traditional Architecture Decision Records (ADRs) that accumulate dead historical decisions and pollute AI context windows, living specifications remain 100% current. Historical evolution is recorded cleanly in `CHANGELOG.md` and Git commit logs.

### Dependency & Blast-Radius Scoping
- Every living specification must document its Dependency & Blast-Radius Matrix (upstream callers, downstream dependencies, and affected packages).
- When resolving bugs, refactoring, or extending existing code, AI agents inspect the living spec's blast-radius matrix to strictly isolate their investigation and edits, eliminating wasteful full-codebase scans.

## Operational Lifecycle Gates [จุดตรวจวงจรการทำงาน]

### GATE 0: MISSION APPROVAL (The Hard Stop) [จุดตรวจอนุมัติภารกิจ]
- **Anti-Hallucination Discovery**: Empirically verify library versions, external APIs, and project configurations via terminal commands or official docs before proposing solutions. Never guess dependencies or symbols.
- Research the task using read-only operations.
- Produce or update the living specification in `docs/specs/<feature>.md` when defining or altering feature logic.
- Produce a structured implementation plan describing proposed technical changes, demarcating modified files and explicit verification steps.
- **Hard Stop**: Halt execution and wait for explicit human green light (Directive Mode) before touching codebase files. Refine the plan if feedback or counter-proposals are given.

### GATE 1: SELF-VERIFICATION & SURGICAL EXECUTION [จุดตรวจการลงมือและยืนยันด้วยตนเอง]
- **Step 0 Branch Isolation**: Before modifying, creating, or deleting any codebase file, verify `git branch --show-current`. If on `main`, immediately execute `git checkout -b <type>/<slug>`. Working directly on `main` is strictly prohibited.
- **Incremental Micro-Commits**: Save commits in small, logical, atomic increments as intermediate milestones are verified. Never hold large uncommitted changes until final completion.
- Make minimal, modular edits focused strictly on the approved scope.
- **Targeted Verification**: Check `git status` before running verification commands. Execute project native test suites and linters via Terminal to verify Exit Code 0 and zero regression.

### GATE 2: KNOWLEDGE Capture & PARITY [จุดตรวจบันทึกความรู้และสเปกมีชีวิต]
- **Code-Spec Parity Verification**: Ensure code implementations match living specs in `docs/specs/`.
- **Evidence Attachment**: Attach empirical test execution logs demonstrating clean passing results (Exit Code 0).
- **Task State Synchronization**: Before concluding any session or task, the agent MUST update `TASK.md` in the project root to reflect the newly completed milestones and immediate next actions.
- **Task State Pruning**: The `Completed Milestones` section in `TASK.md` acts as a short-term buffer. It MUST be cleared/pruned immediately after a formal Release is cut (and recorded in `CHANGELOG.md`) to prevent infinite file growth.
- Provide a concise walkthrough of changes and test results, then conclude the task cleanly.

## Definition of Done (DoD)

A task is considered complete only when all the following criteria are satisfied:
- **Evidence-Based Completion**: Concrete empirical evidence (passing test suite output, compiler logs, or behavioral confirmation) must be produced. A task is never marked done based solely on code generation without verification.
- **Blast-Radius Verification**: Verification commands strictly matched the modified surfaces without executing unrelated checkers.
- All unit and integration tests pass without failure.
- Static analysis and code formatting checks pass cleanly.
- New or modified logic includes adequate test coverage.
- Related documentation (`ARCHITECTURE.md`, `DESIGN.md`, or `docs/`) is synchronized.
- Git commit messages comply with project commit conventions.
