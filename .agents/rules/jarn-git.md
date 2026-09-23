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

- **Atomic Milestones**: Commit code incrementally as each cohesive sub-task is completed and verified. Do not accumulate large, uncommitted diffs across multiple components.
- **Granular Checkpointing**: Save commits after completing logical units of work (such as introducing a failing test, implementing a specific helper, or refining documentation). This creates safe rollback points and allows comparing behavioral changes across intermediate stages.
- **Clean Context Switching**: Working in isolated branches with frequent commits ensures an engineer or agent can switch contexts to address an urgent production hotfix immediately without losing in-flight feature work.
- **No Big-Bang Commits**: Committing an entire days-long or multi-component task in a single monolithic commit at the end of development is strictly prohibited.
