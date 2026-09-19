---
name: jarn-framework-update
description: >-
  Update the installed Jarn framework, analyze changes to shared standards, and
  identify required project migrations.
---

# Jarn Framework Update & Shadow Merge [คู่มือการอัพเดท framework และผสานโค้ด]

This skill defines the approved procedure to update the Jarn framework (rules, skills, templates) and perform an AI-driven Shadow Merge.

## When to Use This Skill [เมื่อใดควรใช้สกิลนี้]

Activate this workflow when:
- The user requests a Jarn framework update (e.g. "update Jarn framework", "/jarn-framework-update").
- A new version of Jarn is known to be available and needs to be pulled.

## Operational Execution Runbook [ขั้นตอนการปฏิบัติงาน]

### Pre-Update Safety Check [ตรวจความพร้อมก่อนอัพเดท]
- Confirm that the user explicitly requested the update or approved a previously presented update plan.
- Run `git status` and stop if unrelated local changes could be overwritten or confused with the update result.
- Verify that work is on an isolated branch before allowing the updater to modify framework files.

### Execute the Updater Script [รันสคริปต์อัพเดทผ่านอินเทอร์เน็ต]
- Run the following command in the terminal to execute the Jarn unified installer:
  `curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/scripts/jarn.sh | sh`
- Wait for the script to finish and check the exit code. If it fails, report the error to the user immediately.

### AI-Driven Shadow Merge [วิเคราะห์และผสาน Shadow Templates]
- The installer has stored the latest Jarn templates inside `.agents/.jarn-templates/`.
- **Your Job as an AI**: You must compare the files in `.agents/.jarn-templates/` with the active files in the project root (e.g., `AGENTS.md`, `REVIEW.md`, `docs/README.md`).
- Identify any missing standards, structural updates, or new invariants introduced in the shadow templates.
- **Intelligently Merge**: Propose and apply updates to the root project files. You MUST strictly preserve existing project-specific commands, configurations, and domain context. Do NOT simply overwrite the root files.
- Ensure that the project root documents do not contain hardcoded copies of Jarn rules (Anti-Duplication Audit). If found, replace them with reference links to the central `.agents/rules/jarn-*.md` files.

### Report and Follow-up [รายงานและดำเนินการต่อ]
- Present a clear summary of what core `.agents/` files were updated.
- Present a summary of the structural changes merged from `.agents/.jarn-templates/` into the project root files.
- Highlight any **breaking changes** or new rules that the project needs to adopt.
- Verify clean integration with `git status` and `git diff`.
