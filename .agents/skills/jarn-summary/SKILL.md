---
name: jarn-summary
description: >-
  This skill analyzes recent chat conversations, living specs (docs/specs/), and active task states (.scratch/ or TASK.md)
  to generate a structured summary of agreed requirements, current task status, open decisions, and immediate next actions.
---

# Jarn Task & Spec Summary Workflow

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

Structured procedure for aggregating, clarifying, and presenting the current active task state, scope deltas from recent conversation history, living spec contracts (`docs/specs/`), and immediate actionable deliverables.

## Core Philosophy

During collaborative development, requirements and specifications evolve across multiple back-and-forth messages, inline comments, and trade-off debates. This skill acts as a single source of truth synthesizer, bringing instant clarity to "What needs to be done right now?" without searching through past chat logs.

## When to Use

Activate when:
- User asks "สรุปรายละเอียดที่ต้องทำ", "What needs to be done?", `/summary`, or `/สรุปรายละเอียดที่ต้องทำ`.
- After extensive brainstorming, design debate, or spec revision in the chat thread.
- Re-orienting on context after context shifts or long conversations.

Do NOT use when:
- Conducting initial requirement discovery (use `jarn-consult` instead).
- Performing automated pre-merge audit (use `jarn-review` instead).

---

## Operational Execution Runbook

### Phase 0: Context Gathering

Scan the workspace to gather current state evidence:
- **Conversation Delta**: Re-read recent chat history to capture agreed changes, user comments, and refined constraints.
- **Living Specifications**: Check `docs/specs/` for active vertical slice specs touched by the current task.
- **Task & Issue State**: Check `.scratch/<task-slug>/plan.md` and `.scratch/<task-slug>/issues/` (or root `TASK.md` if scratch is absent).
- **Git Working Tree**: Inspect `git status` and `git diff` to identify what is modified vs uncommitted.

---

### Phase 1: Synthesize Findings

Structure the collected context into 4 mandatory sections:

```markdown
# 📌 สรุปความต้องการและสเป็กที่ตกลงล่าสุด (Agreed Scope & Revisions)
- [List key agreed functional & technical requirements]
- [Detail any revisions or additions made during the conversation]
- [Reference living specs: `docs/specs/<feature>.md`]

# 📋 รายการงานที่ต้องทำ (Action Items Matrix)
- [x] **[Completed]** [Finished sub-tasks and verified deliverables]
- [ ] **[In Progress]** [Currently active work items]
- [ ] **[Pending]** [Remaining backlog items]

# ❓ ประเด็นรอสรุป / รอตัดสินใจ (Open Decisions & Blockers)
- [List any unresolved trade-offs, pending human sign-offs, or blockages]

# 🚀 ขั้นตอนถัดไป (Immediate Next Actions)
- [Clear prompt specifying exact next action or required directive]
```

---

### Phase 2: Actionable Handoff

Present the synthesized digest cleanly in chat and offer clear next step triggers for the user.
