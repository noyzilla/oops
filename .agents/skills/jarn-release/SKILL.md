---
name: jarn-release
description: End-to-end automated release lifecycle, including SemVer calculation, CHANGELOG drafting, and tag publishing across any Git host.
---

# Jarn Release Lifecycle

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

**Description**: Automates the version release process by analyzing Git commit history, calculating Semantic Versioning (SemVer), updating the `CHANGELOG.md`, and creating formal Git tags or repository releases.

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
- **Commits Exist**: `git log <last_tag>..HEAD --oneline` returns at least one non-chore commit.
- **Remote Configured**: `git remote -v` confirms a valid upstream or origin remote.

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

Create the annotated Git tag and push to remote:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

### Release Mode

Release publication behavior is governed by the `Release Mode` declared in the project `AGENTS.md` (Project Execution Commands). Creating only a tag is never a silent default; it is valid only when the mode is `tag-only`.

| Mode | Meaning | Done when |
| :--- | :--- | :--- |
| `tag-only` | Release means declaring the version shipped. No release object, no CI. | Tag exists on remote. |
| `host-release` | A release object must exist on the repository host. | Release view command exits 0. |
| `ci-release` | CI creates the release from the pushed tag. | CI-created release is visible on the host. |

#### Mode Resolution
- Read `Release Mode` from `AGENTS.md`. If declared, use it.
- If missing or still a template placeholder, infer from the remote host (`git remote get-url origin`) and existing CI release workflows (for example `.github/workflows/*release*`, `.gitlab-ci.yml`).
  - **Trigger Level 1**: Include the inferred mode in the Execution Plan and wait for user confirmation.
  - **Trigger Level 2**: Halt if the mode cannot be inferred unambiguously. Never fall back to tag-only silently.

#### Host Release Commands (`host-release`)
Use the CLI matching the remote host, with notes extracted from the matching version section of `CHANGELOG.md` (write to a temporary file under `.scratch/`):
- **GitHub**: `gh release create vX.Y.Z --title "Release vX.Y.Z" --notes-file <notes>` (attach built `dist/` or `bin/` assets if applicable).
- **GitLab**: `glab release create vX.Y.Z --notes-file <notes>`
- **Gitea / Forgejo**: `tea releases create --tag vX.Y.Z --title "Release vX.Y.Z" --note-file <notes>`
- **Other hosts**: Halt and ask the user.

If the CLI is missing or unauthenticated, halt and report. Do not downgrade to tag-only.

#### CI Release (`ci-release`)
Push the tag, then wait for the CI pipeline and verify the release object. Do not create the release manually, to avoid duplicates.

### Verification
- Verify the tag exists locally and remotely via `git tag -l vX.Y.Z` and `git ls-remote --tags origin vX.Y.Z`.
- Verify the release object per mode: `gh release view vX.Y.Z` or `glab release view vX.Y.Z` (`host-release` and `ci-release`).
- Report the release version, tag, resolved mode with its source (declared or inferred), and publish confirmation to the user.
