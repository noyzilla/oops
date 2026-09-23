# Contributing Guidelines

Thank you for contributing to this project. All contributors (human engineers and AI coding assistants) operate under the same engineering standards to ensure high architectural integrity.

## AI Usage Policy & Contributor Accountability

Contributions prepared with the assistance of generative AI tools are welcomed, under the following conditions:

- **Full Contributor Accountability**: The submitter is 100% responsible for every line of code, test, and documentation in the pull request.
- **No Incomplete AI Slop**: Pull requests containing incomplete implementations, speculative changes, unverified code, or starter scaffolding expecting others to finish will be closed immediately.
- **Focused Scope**: Submit single, focused changes linked to an approved issue or task. Do not include unrelated reformatting or stylistic churn.
- **Verification Mandatory**: All contributions must include test coverage and pass all automated verification checks before submission.

## Engineering Standards

This repository enforces unified engineering conventions:

- For commit formats and SemVer rules, see [.agents/rules/jarn-git.md](.agents/rules/jarn-git.md#commit-conventions).
- For documentation and commenting rules, see [.agents/rules/jarn-coding.md](.agents/rules/jarn-coding.md#documentation--commenting-rules).
- For ecosystem lifecycle guidelines, see [.agents/rules/jarn-architecture.md](.agents/rules/jarn-architecture.md#ecosystem-native-lifecycle-contract).

## Project Team & Roles (Handoff Protocol)

This project supports decentralized collaboration. Tasks flow continuously to completion (Single-Flow) unless a handoff is required. When handing off work to another team member or agent, use the `handoff(<target>): <message>` commit convention.

Find your role below to understand your triggers and responsibilities:

- **Role: `dev` (Software Engineers)**
  - **Trigger**: New task assigned.
  - **Action**: Branch out, create branch-scoped `TASK.md`, implement code, and write unit tests.
  - **Handoff**: Commit `handoff(qa): ready for UI tests` and push to origin. (If no QA is required, proceed to merge).

- **Role: `qa` (Quality Assurance)**
  - **Trigger**: Wait for a commit starting with `handoff(qa): ...`
  - **Action**: Pull the branch, run E2E/UI tests, and update the branch's `TASK.md`.
  - **Handoff**: Commit `handoff(reviewer): all tests passed` (or handoff back to dev if failed).

- **Role: `reviewer` (Code Owners)**
  - **Trigger**: Wait for a commit starting with `handoff(reviewer): ...`
  - **Action**: Perform Pre-Merge Audit (GATE 3).
  - **Handoff**: Merge pull request to `main` and delete branch.

## Pull Request Workflow [ขั้นตอนการส่งงาน Pull Request]

### Branch Isolation & Lifecycle
All work must be conducted within isolated branches branched from `main`. Direct commits or pushes to `main` are strictly forbidden (Step 0 Invariant). See [.agents/rules/jarn-governance.md](.agents/rules/jarn-governance.md).

### Incremental Micro-Commits
See [.agents/rules/jarn-git.md](.agents/rules/jarn-git.md#commit-frequency--granularity-micro-commit-strategy).

### Pre-Submission Verification
Before opening a pull request, run the active verification commands configured for this project in [AGENTS.md](AGENTS.md#project-execution-commands).

### Pre-Merge Quality Checklist
Verify that your pull request satisfies all quality gates defined in the pre-merge checklist:
- Universal Pre-Merge Quality Gates: [.agents/rules/jarn-quality.md](.agents/rules/jarn-quality.md)
- Project Pre-Merge Checklist: [REVIEW.md](REVIEW.md)

---

## Local Development

### Prerequisites
- [Go 1.22+](https://golang.org/)
- [Docker](https://www.docker.com/)

### Running Tests
Execute all unit tests across modules:
```bash
go test -v ./...
```

### Building the Binary
```bash
go build -o oops .
```

---

## Automated AI Release Process [ขั้นตอนการ Release โดย AI]

Releases are fully automated by AI agents to ensure meticulous changelog generation and correct semantic versioning. 

Instead of manually calculating the next version and drafting changelogs, simply instruct the AI agent to trigger a release (e.g., `"ทำการ Release เวอร์ชั่นใหม่"` or `/goal Run jarn-release`).

The agent will activate the `jarn-release` skill, which will:
- Analyze all Conventional Commits since the last tag to calculate the next SemVer (Major/Minor/Patch).
- Generate a highly detailed `CHANGELOG.md` entry with context.
- Commit the changelog and push the new Git tag automatically.
