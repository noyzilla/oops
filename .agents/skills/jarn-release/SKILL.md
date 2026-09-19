---
name: jarn-release
description: End-to-end automated GitHub Release lifecycle, including SemVer calculation, CHANGELOG drafting, and tag publishing.
---

# Jarn Release Lifecycle [วงจรการเผยแพร่ Jarn]

**Description**: Automates the version release process by analyzing Git commit history, calculating Semantic Versioning (SemVer), updating the `CHANGELOG.md`, and publishing a formal GitHub Release via the GitHub CLI (`gh`).

**Primary Objective**: To ensure releases are created safely and consistently without manual version guessing or manual changelog writing.

---

## Release Readiness [ความพร้อมก่อนเผยแพร่]

Before preparing or publishing a release, verify the following:
- **Clean Workspace**: Run `git status`. The working directory must be completely clean.
- **GitHub CLI**: Verify `gh` is installed (`command -v gh`).
- **Release Authority**: Confirm the user requested a release or explicitly approved a release plan.

---

## Release Preparation [การเตรียม release]

Prepare a release on an isolated `chore/release-<version>` branch. Do not commit directly to `main`.

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
- Commit the changelog on the isolated release branch using `chore(release): vX.Y.Z`.
- Submit the branch through the project's normal review and merge process.
- Do not push directly to `main`.

## Post-Release Cleanup [การทำความสะอาดหลังเผยแพร่]

Once the release is merged and published, the working context must be reset:
- **Clean TASK.md**: Clear all items under the `Completed Milestones` section in `TASK.md`. Those milestones are now permanently recorded in the `CHANGELOG.md`, and `TASK.md` should be reset for the next iteration to prevent infinite file growth.

## Publication Approval [การอนุมัติเผยแพร่]

- After the release preparation is merged, check out the clean `main` branch and verify the approved changelog commit is present.
- Present the exact version, target commit, and release title to the user.
- Wait for explicit approval before creating the tag or publishing a GitHub Release.

## Publish and Verify [เผยแพร่และตรวจสอบ]

After publication approval, use the GitHub CLI to create the release and tag simultaneously:
```bash
gh release create vX.Y.Z --generate-notes --title "Release vX.Y.Z"
```
*(Using `--generate-notes` leverages GitHub's automatic release notes generation based on merged PRs, ensuring a clean release payload).*

- Verify the release was created successfully by checking `gh release view vX.Y.Z`.
- Report the release URL, tag, and verification result to the user.
