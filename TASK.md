# Branch Task: [Task Title]

This file serves as the state machine for the current branch. It tracks the progress of the feature or fix from development through testing and Pre-Merge Audit.

- **Branch Target**: `[branch-name]`
- **Related Specs**: [Link to docs/specs/ here]

## Dev Execution (GATE 2)
*Completed by the Developer / Dev Agent.*

- [ ] Implemented core logic according to the living specifications.
- [ ] Added or updated unit tests to cover new behavior.
- [ ] Passed all local verification checks (tests, linters).
- [ ] **Handoff (If required):** If the workflow requires another role (e.g., QA) to test or review, execute `git commit -m "handoff(<target>): ready for review"` and push to origin.

## QA & Review (GATE 3)
*Completed by the QA / Reviewer Agent (if applicable).*

- [ ] Pulled branch and verified behavior against requirements.
- [ ] Verified UI/UX elements and edge cases.
- [ ] Completed the Pre-Merge Quality Gate checklist in `REVIEW.md`.
- [ ] **Approval:** Executed `git commit -m "handoff(merge): qa passed, ready for merge"`.

## Pre-Merge Cleanup
- [ ] Before merging the pull request, clear these checklists or delete this file to keep the `main` branch clean.
