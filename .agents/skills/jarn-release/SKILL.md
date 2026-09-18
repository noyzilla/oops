---
name: jarn-release
description: End-to-end automated GitHub Release lifecycle, including SemVer calculation, CHANGELOG drafting, and tag publishing.
---

# `jarn-release` Skill

**Description**: Automates the version release process by analyzing Git commit history, calculating Semantic Versioning (SemVer), updating the `CHANGELOG.md`, and publishing a formal GitHub Release via the GitHub CLI (`gh`).

**Primary Objective**: To ensure releases are created safely and consistently without manual version guessing or manual changelog writing.

---

## 🚦 Pre-Flight Guard (GATE 0)

Before executing the release process, you MUST verify the following:
1. **Clean Workspace**: Run `git status`. The working directory must be completely clean. Do NOT proceed if there are uncommitted changes.
2. **Current Branch**: Run `git branch --show-current`. You MUST be on `main` (or the trunk branch). Do NOT release from a feature branch.
3. **GitHub CLI**: Verify `gh` is installed (`command -v gh`).

---

## 🛠 Execution Workflow

If the Pre-Flight Guard passes, follow these steps strictly:

### 1. Identify the Previous Tag
Run `git tag --sort=-v:refname | head -n 1`.
- If a tag exists, use it as the `<last_tag>`.
- If no tags exist, the previous state is from the beginning of the repository. Set `<last_tag>` to empty.

### 2. Analyze Commits and Calculate SemVer
Run `git log <last_tag>..HEAD --oneline` (or `git log --oneline` if no previous tag exists).
Analyze the commits using the Conventional Commits specification:
- **Major Bump**: If any commit contains `BREAKING CHANGE:` or an `!` (e.g., `feat!:`).
- **Minor Bump**: If any commit starts with `feat:`.
- **Patch Bump**: If commits are only `fix:`, `docs:`, `chore:`, `refactor:`, etc.
*Determine the new version string (e.g., `v0.1.3`).*

### 3. Draft the Changelog
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

### 4. Commit and Push
1. Run `git add CHANGELOG.md`.
2. Run `git commit -m "chore(release): vX.Y.Z"`.
3. Run `git push origin main`.

### 5. Publish the GitHub Release
Use the GitHub CLI to create the release and tag simultaneously.
Run:
```bash
gh release create vX.Y.Z --generate-notes --title "Release vX.Y.Z"
```
*(Using `--generate-notes` leverages GitHub's automatic release notes generation based on merged PRs, ensuring a clean release payload).*

---

## 🛑 Verification (GATE 1)

1. Verify the release was created successfully by checking `gh release view vX.Y.Z`.
2. Inform the user that the release has been successfully deployed and the Jarn blueprint distribution is now updated.
