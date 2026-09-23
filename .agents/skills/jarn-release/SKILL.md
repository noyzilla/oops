---
name: jarn-release
description: End-to-end automated GitHub Release lifecycle, including SemVer calculation, CHANGELOG drafting, and tag publishing.
---

# Jarn Release Lifecycle

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

**Description**: Automates the version release process by analyzing Git commit history, calculating Semantic Versioning (SemVer), updating the `CHANGELOG.md`, and publishing a formal GitHub Release via the GitHub CLI (`gh`).

**Primary Objective**: To ensure releases are created safely and consistently without manual version guessing or manual changelog writing.

---

## Composite Intent & Trigger Levels

The word `release` is a composite command that implies completing the full lifecycle. Two trigger levels determine agent behavior:

### Trigger Level 1: `release` (Inform & Confirm)
When the user issues `release` without a finalization qualifier, the agent MUST:
1. Assess the current workspace state (branch, uncommitted changes, pending merges).
2. Analyze commits since the last tag and calculate the SemVer bump.
3. Present a numbered **Execution Plan** summarizing every step the agent will perform, including pre-condition resolution, review, merge, changelog update, and publication.
4. **Wait for explicit user approval** before executing any step.

### Trigger Level 2: `จบงาน release` / `ปิดจบ release` (Execute Full Pipeline)
When the user issues a finalization-qualified release directive, the agent executes the full pipeline autonomously:
1. Resolve all pre-conditions (commit, review, merge) automatically.
2. Perform the release on `main` (CHANGELOG, tag, publish).
3. **Quality gates are never skipped** — if any gate fails, halt and report the failure.

---

## Pre-condition Resolution

Before release preparation can begin, the workspace must be on a clean `main` branch. When the release command is issued from a non-ideal state, the agent resolves pre-conditions in order:

### Uncommitted Changes
If `git status` shows modified or untracked files relevant to the current branch's work:
- Stage and commit them with an appropriate Conventional Commit message.
- Do NOT commit unrelated or leftover files without confirming intent.

### Non-Main Branch
If the current branch is not `main`:
- Activate and execute the `jarn-review` skill for pre-merge quality verification.
- If review passes, merge the branch into `main` (fast-forward or merge commit as appropriate).
- If review fails, halt the pipeline and report failures to the user.

### Dirty Main
After switching to `main`, verify `git status` is clean. If not, halt and report.

---

## Release Readiness

After pre-conditions are resolved and the agent is on a clean `main`, verify:
- **GitHub CLI**: `command -v gh` confirms `gh` is available.
- **Commits Exist**: `git log <last_tag>..HEAD --oneline` returns at least one non-chore commit.

---

## Release Preparation (on `main`)

Release preparation commits directly on `main`. This is an explicit exception to the Step 0 Branch Isolation rule, as `chore(release):` commits contain only mechanical changelog and metadata changes with zero logic risk.

### Identify the Previous Tag
Run `git tag --sort=-v:refname | head -n 1`.
- If a tag exists, use it as the `<last_tag>`.
- If no tags exist, the previous state is from the beginning of the repository. Set `<last_tag>` to empty.

### Analyze Commits and Calculate SemVer
Run `git log <last_tag>..HEAD --oneline` (or `git log --oneline` if no previous tag exists).
Analyze the commits using the Conventional Commits specification:
- **Major Bump**: If any commit contains `BREAKING CHANGE:` or an `!` (e.g., `feat!:`).
- **Minor Bump**: If any commit starts with `feat:`.
- **Patch Bump**: If commits are only `fix:`, `docs:`, `chore:`, `refactor:`, etc.
*Determine the new version string (e.g., `v0.1.3`).*

### Draft the Changelog
Read the current `CHANGELOG.md`.
Insert a new section at the top (under the main header) for the new version:

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Features
- ... (extract from `feat:` commits)

### Fixes
- ... (extract from `fix:` commits)
```
*Filter out internal chores or noise if they aren't relevant to end users, but keep the changelog accurate based on the git log.*

### Commit Release Preparation
- Commit the changelog on `main` using `chore(release): vX.Y.Z`.

---

## Post-Release Cleanup

Once the release is published, the working context must be reset:
- **Clean TASK.md**: Clear all items under the `Completed Milestones` section in `TASK.md`. Those milestones are now permanently recorded in the `CHANGELOG.md`, and `TASK.md` should be reset for the next iteration to prevent infinite file growth.

---

## Publication Approval

- For **Trigger Level 1**: Present the exact version, target commit, and release title to the user. Wait for explicit approval before creating the tag or publishing.
- For **Trigger Level 2**: Proceed directly to publication after the changelog commit. The finalization directive already constitutes approval.

---

## Publish and Verify

Use the GitHub CLI to create the release and tag simultaneously:
```bash
gh release create vX.Y.Z --generate-notes --title "Release vX.Y.Z"
```
*(Using `--generate-notes` leverages GitHub's automatic release notes generation based on merged PRs, ensuring a clean release payload).*

- Verify the release was created successfully by checking `gh release view vX.Y.Z`.
- Report the release URL, tag, and verification result to the user.
