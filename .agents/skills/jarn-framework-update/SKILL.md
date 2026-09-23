---
name: jarn-framework-update
description: >-
  Update the installed Jarn framework, reload updated rules and skills, analyze changes to shared standards, and
  execute an AI-driven Shadow Merge on project files.
---

# Jarn Framework Update & Shadow Merge

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the approved procedure to update the Jarn framework (rules, skills, templates), reload newly installed standards, and perform an AI-driven Shadow Merge.

## When to Use This Skill

Activate this workflow when:
- The user requests a Jarn framework update (e.g. "update Jarn framework", "/jarn-framework-update").
- A new version of Jarn is known to be available and needs to be pulled.

## Operational Execution Runbook

### Pre-Update Safety Check
- Confirm that the user explicitly requested the update or approved a previously presented update plan.
- Run `git status` and stop if unrelated local changes could be overwritten or confused with the update result.
- Verify that work is on an isolated branch (`feat/...`, `chore/...`) before allowing the updater to modify framework files.

### Execute the Updater Script
- Run the following command in the terminal to execute the Jarn unified installer:
  `curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/scripts/jarn.sh | sh`
- Wait for the script to finish and check the exit code. If it fails, report the error to the user immediately.

### Ingest Updated Rules & Modular Skills
- **Re-Discover & Ingest Rules**: Immediately after the script completes, execute directory discovery on `.agents/rules/` to discover all active `jarn-*.md` files. Read each newly installed rule file to internalize all updated invariants, new lifecycle gates, renamed files (e.g. `jarn-quality.md`), and governance changes.
- **Discover & Ingest Skills**: Execute directory discovery on `.agents/skills/` to discover all installed `jarn-*` skills. Read their `SKILL.md` files to understand new capabilities (such as `jarn-consult` for GATE 1 requirement discovery and Socratic brainstorming) and updated runbooks.
- **Mental Model Realignment**: Ensure your operational context is 100% aligned with the newly installed framework standards BEFORE attempting to modify any project root files.

### AI-Driven Shadow Merge
- The installer has stored the latest Jarn templates inside `.agents/.jarn-templates/`.
- **Your Job as an AI**: Compare the templates in `.agents/.jarn-templates/` with the active files in the project root (e.g., `AGENTS.md`, `REVIEW.md`, `CONTRIBUTING.md`, `docs/README.md`).
- Identify any missing standards, structural updates, renamed rule references, or new invariants introduced in the shadow templates.
- **Intelligently Merge**: Propose and apply updates to the root project files. You MUST strictly preserve existing project-specific commands, configurations, and domain context. Do NOT simply overwrite the root files.
- **Context-Aware Merging (e.g., DESIGN.md)**: Before merging files like `DESIGN.md`, study the project's existing content to determine its context. If the project is a CLI, library, or backend service, DO NOT force web/UI design tokens into it. Only merge relevant updates (like CLI log formats) and preserve the project's original intent.
- Ensure that the project root documents do not contain hardcoded copies of Jarn rules (Anti-Duplication Audit). If found, replace them with reference links to the central `.agents/rules/jarn-*.md` files.

### Report and Follow-up
- Present a clear summary of what core `.agents/` files were updated.
- Present a summary of the structural changes merged from `.agents/.jarn-templates/` into the project root files.
- Highlight any **breaking changes** or new rules that the project needs to adopt.
- Verify clean integration with `git status` and `git diff`.
