# Branch Task: Jarn Framework Update to v0.6.0

This file serves as the state machine for the current branch. It tracks the progress of the feature or fix from development through testing and Pre-Merge Audit.

- **Branch Target**: `chore/jarn-framework-update`
- **Related Specs**: N/A (Framework Update)

## Dev Execution (GATE 2)
*Completed by the Developer / Dev Agent.*

- [x] Run Jarn updater script (`curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/scripts/jarn.sh | sh`).
- [x] Synchronize Jarn rules (`jarn-lifecycle.md`, `jarn-quality.md`) and skills (`jarn-consult`).
- [x] Execute AI-Driven Shadow Merge between `.agents/.jarn-templates/` and root project files (`AGENTS.md`, `REVIEW.md`, `docs/README.md`, `ARCHITECTURE.md`, `CONTRIBUTING.md`, `.gitignore`).
- [x] Pass all local verification checks (tests, linters).

## QA & Review (GATE 3)
*Completed by the QA / Reviewer Agent (if applicable).*

- [ ] Completed the Pre-Merge Quality Gate checklist in `REVIEW.md`.
- [ ] Run targeted verification (`git diff --check`, `go test ./...`, etc.).

## Pre-Merge Cleanup
- [ ] Before merging the pull request, clear these checklists or reset to template to keep the `main` branch clean.
