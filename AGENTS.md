# Agent Operational Guide [คู่มือการปฏิบัติงานสำหรับเอไอ]

> **ALWAYS ON DIRECTIVE: MUST READ CORE RULES**
>
> You are operating under the Jarn Governance Framework. These rules are **Always ON** and non-negotiable.
> **You MUST read** the core rule files before executing any task. Do not assume you already know them.
> First, use your directory listing tool on `.agents/rules/` to discover all active `jarn-*.md` files. Then, read each of them.
>
> **MANDATORY PRE-FLIGHT GUARD [มาตรฐานจุดตรวจก่อนเริ่มงาน]**:
> - **Step 0 Branch Isolation**: NEVER edit or commit on `main`. Verify `git branch --show-current` before modifying any files. Branch out (`git checkout -b <type>/<slug>`) immediately if on `main`. (See [.agents/rules/jarn-governance.md](.agents/rules/jarn-governance.md))
> - **Inquiry vs Directive**: Treat discussions as Inquiry Mode (read-only analysis). Do NOT mutate code without an explicit Directive trigger (e.g. "ทำเลย", "อนุมัติ", "proceed"). (See [.agents/rules/jarn-governance.md](.agents/rules/jarn-governance.md))
> - **Operational Workflow Gates**: Adhere strictly to **GATE 1** (Living Spec & Mission Approval) -> **GATE 2** (Surgical Execution & Self-Verification) -> **GATE 3** (Knowledge Capture & Pre-Merge Audit).

This document is the primary machine-readable entrypoint for AI coding agents collaborating on this codebase.

## Current Rule Manifest [รายการกฎที่ใช้งานอยู่]

Directory discovery in the pre-flight guard determines the active rules. This manifest describes the expected Jarn baseline and MUST remain synchronized with `.agents/rules/jarn-*.md`.

Agents MUST strictly comply with every active Jarn rule:

- **Governance & Safety**: [.agents/rules/jarn-governance.md](.agents/rules/jarn-governance.md) (Safety boundaries, escalation gates, workflow state machine, change taxonomy).
- **Operational Lifecycle**: [.agents/rules/jarn-lifecycle.md](.agents/rules/jarn-lifecycle.md) (GATE 1 → GATE 2 → GATE 3 operational gates, gate checklists, Definition of Done).
- **Architecture & Lifecycle**: [.agents/rules/jarn-architecture.md](.agents/rules/jarn-architecture.md) (Ecosystem-native lifecycle, configuration architecture, document splitting, frontmatter).
- **Naming Conventions**: [.agents/rules/jarn-naming.md](.agents/rules/jarn-naming.md) (Path notation, environment variable parity, domain alignment).
- **Coding Style & Hygiene**: [.agents/rules/jarn-coding.md](.agents/rules/jarn-coding.md) (Visual hygiene, bilingual annotations, numbering invariants, stable references).
- **Git & Commits**: [.agents/rules/jarn-git.md](.agents/rules/jarn-git.md) (Conventional commits, micro-commit strategy).
- **Quality & Pre-Merge Gates**: [.agents/rules/jarn-quality.md](.agents/rules/jarn-quality.md) (Pre-merge review checklist, universal quality standards, targeted verification).
- **Local Domain Context**: Align all entity names and parameters with ubiquitous language in [CONTEXT.md](CONTEXT.md).
- **Documentation Taxonomy**: Maintain system documentation under `docs/` according to [docs/README.md](docs/README.md).

## Project Execution Commands

Configure the active lifecycle commands below for your specific tech stack. Run verification checks before completing any task.

### Active Commands
- **Setup Dependencies**: `go mod download && go mod verify`
- **Run Unit Tests**: `go test -v -race ./...`
- **Run UI / E2E Tests**: N/A
- **Run Linter / Style**: `golangci-lint run`
- **Build Project**: `go build -v -o oops .`
- **Run Local Dev**: `go run main.go`

## Change Routing Matrix

When modifying specific layers or subsystems, update the designated locations and run the targeted verification command:

| Change Area | Primary Files to Update | Verification Command |
| :--- | :--- | :--- |
| **Domain Logic & Services** | `src/domain/`, `src/services/`, `docs/specs/` | Run unit tests for affected package |
| **API & Routing Layer** | `src/api/`, `src/controllers/`, `routes/`, `docs/specs/` | Run API / integration tests and linter |
| **Data Schema & Migrations** | `migrations/`, `db/`, `models/`, `docs/specs/` | Run migration scripts and database test suite |
| **CLI Output & Logging** | `DESIGN.md`, `docs/design/` | `git diff --check` and verify log formatting rules |
| **Living Specifications** | `docs/specs/` | `git diff --check` and verify spec contract alignment |
| **Architectural Decisions (ADR)** | `docs/decisions/` | `git diff --check` and verify filename status lifecycle |
| **Development & Runbooks** | `docs/development/` | `git diff --check` and test script execution |
| **Automation & Shell Scripts** | `scripts/*.sh`, `.agents/scripts/*.sh` | `sh -n <touched_script>` and dry-run execution |
| **Universal Standards & Rules** | `.agents/rules/jarn-*.md`, `AGENTS.md` | `git diff --check` and verify Markdown links |
| **Jarn Skills** | `.agents/skills/jarn-*` | `git diff --check` and verify skill frontmatter/links |
| **Documentation Only** | `docs/`, `*.md` | `git diff --check` and verify Markdown links |

### Verification Policy for Agents
- **Targeted Verification Invariant (No Blind Runs)**: Always check `git status` before executing verification commands. Never run syntax checkers (e.g., `sh -n`), language linters, or test suites for file extensions or components that were not modified in the current changeset.
- **Iterative Check**: Run the narrowest relevant check while actively developing logic.
- **Pre-Completion Gate**: Run the full verification checks specified in the Change Routing Matrix strictly for touched surfaces.
- **Mandatory Pre-Merge Audit Execution**: Before declaring any task complete or concluding a branch, the agent MUST activate and fulfill [.agents/skills/jarn-review](.agents/skills/jarn-review/SKILL.md).
- **Pre-Merge Audit Checklist**: Ensure all items in [REVIEW.md](REVIEW.md) are satisfied.

---

## Project Context

- **Project Name**: oops
- **Primary Language / Runtime**: go
- **Architecture Pattern**: webhook handler

## Available Modular Skills

When specialized expertise or operational procedures are required, activate the relevant skill under `.agents/skills/`:
- [jarn-consult](.agents/skills/jarn-consult/SKILL.md): Structured requirement discovery and brainstorming (GATE 1). Classifies work, coaches intent with focused questions, evaluates viability, proposes implementation options with trade-offs, and hands off a confirmed agreement to jarn-spec.
- [jarn-spec](.agents/skills/jarn-spec/SKILL.md): Author, debate, and maintain vertical slice living specifications (`docs/specs/`) with Code-Spec Parity and blast-radius matrices (GATE 1).
- [jarn-decisions](.agents/skills/jarn-decisions/SKILL.md): Manage the lifecycle of Architectural Decision Records (`docs/decisions/`) with zero-token filtering.
- [jarn-review](.agents/skills/jarn-review/SKILL.md): Autonomous quality gate runbook (GATE 3) to inspect git status, run targeted verification, audit commit conventions, and synthesize pre-merge evidence against jarn-quality.md.
- [jarn-diagnostics](.agents/skills/jarn-diagnostics/SKILL.md): Isolated defect investigation procedure bounded strictly to the living spec's blast-radius matrix without blind codebase scans.
- [jarn-release](.agents/skills/jarn-release/SKILL.md): End-to-end automated GitHub Release lifecycle, including SemVer calculation, CHANGELOG drafting, and tag publishing.
- [jarn-framework-update](.agents/skills/jarn-framework-update/SKILL.md): Update the installed Jarn framework, execute Shadow Merge, and identify required project migrations.
