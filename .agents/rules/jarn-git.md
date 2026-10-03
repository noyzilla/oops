---
trigger: always_on
description: "Git branch isolation, conventional commits, and micro-commit strategy."
---

# Git & Commit Conventions

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Add project-specific git workflows to your project's `docs/development/` only.

This project strictly follows the Conventional Commits specification coupled with Semantic Versioning (SemVer) and Micro-Commit strategies.

## Commit Conventions

### Format
```text
<type>(<scope>): <subject>
```

### Subject Rules
- **Imperative mood**: Use verbs like `add`, `fix`, `refactor` rather than `added` or `fixes`.
- **Lowercase**: Start the subject with a lowercase letter.
- **No trailing period**: Do not place a period at the end of the subject line.

### Commit Types & SemVer Impact

| Prefix | Description | SemVer Bump | Example |
| :--- | :--- | :---: | :--- |
| **`feat:`** | Introduces a new feature | **MINOR** (`v0.1.0` -> `v0.2.0`) | `feat(auth): add jwt validation middleware` |
| **`fix:`** | Fixes a bug | **PATCH** (`v0.1.0` -> `v0.1.1`) | `fix(db): handle connection timeout gracefully` |
| **`feat!:`** or `BREAKING CHANGE:` | Introduces breaking changes | **MAJOR** (`v0.1.0` -> `v1.0.0`) | `feat!(api): require token header on all routes` |
| **`docs:`** | Documentation changes only | None / Patch | `docs: update deployment architecture guide` |
| **`refactor:`** | Code change that neither fixes a bug nor adds a feature | None / Patch | `refactor: simplify target validation` |
| **`test:`** | Adding or updating tests | None | `test: add unit tests for token parser` |
| **`chore:`** | Tooling, build scripts, or maintenance | None | `chore: update build script dependencies` |
| **`handoff:`** | Handing off a task to another role in a Multi-Role Topology | None | `handoff(qa): ready for UI tests` |

## Commit Frequency & Granularity (Micro-Commit Strategy)

To ensure code stability, bisectability, and rapid troubleshooting, all contributors and agents must follow an incremental micro-commit workflow:

- **Working Tree Loop by Default**: Keep all intermediate implementation, edits, and review adjustments in the working tree uncommitted by default across all tasks. Never commit rapid, microscopic adjustments that create noise in the Git history.
- **Atomic Sub-task Milestones**: Commit code incrementally as each cohesive sub-task milestone is completed, verified, and reviewed. When a task comprises multiple sub-tasks, do not advance to the next sub-task before the previous sub-task is confirmed and committed.
- **Review Before Commit**: In dev pairing, present the uncommitted diff and verification logs for reviewer inspection. Execute `git commit` only upon explicit confirmation, unless an auto-commit directive was given upfront.
- **No Fixup Noise (Amend Invariant)**: If adjustments are requested on a recently completed commit within the same branch, amend or soft-reset (`git reset --soft HEAD~1`) to maintain clean, atomic commits rather than stacking fragmented fixup commits.
- **Granular Checkpointing**: Save commits after completing logical units of work (such as introducing a failing test, implementing a specific helper, or refining documentation). This creates safe rollback points and allows comparing behavioral changes across intermediate stages.
- **Clean Context Switching**: Working in isolated branches with frequent commits ensures an engineer or agent can switch contexts to address an urgent production hotfix immediately without losing in-flight feature work.
- **No Big-Bang Commits**: Committing an entire days-long or multi-component task in a single monolithic commit at the end of development is strictly prohibited.
