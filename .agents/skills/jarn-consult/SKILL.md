---
name: jarn-consult
description: >-
  Activate before any spec-altering change to lead structured requirement discovery and brainstorming (GATE 1).
  Classifies the request, probes intent with focused questions, acts as a Socratic coach for junior developers,
  proposes implementation options with trade-offs, and hands off a confirmed agreement to jarn-spec for living spec synthesis.
---

# Jarn Consultation Workflow

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill).

This skill defines the structured requirement discovery and brainstorming procedure executed during GATE 1 for all spec-altering changes. It ensures the AI leads the inquiry and brainstorming — not the human — coaching junior developers, evaluating viability, and producing well-scoped living specs regardless of AI model.

## Core Philosophy

A living specification can only be as good as the requirements behind it. This skill exists to surface what the human has not yet articulated: edge cases, constraints, implicit assumptions, and architectural trade-offs. The AI acts as "จารย์" — an expert advisor and Socratic coach who asks thought-provoking questions, explores creative alternatives, and filters non-viable ideas before prescribing a solution.

## When to Use This Skill

Activate this workflow when:
- Starting a new feature, subsystem, or capability (spec-altering change in GATE 1).
- Requirements feel unclear, incomplete, or contain implicit assumptions.
- A junior developer is initiating work and benefits from proactive Socratic coaching.
- The request scope is ambiguous — could be a small fix, a large redesign, or a non-viable feature.

Do NOT use this workflow when:
- Fixing a bug where the existing spec is already correct (spec-conforming fix — proceed directly to implementation plan in GATE 1 / GATE 2).
- Performing internal refactoring or styling cleanup that preserves external behavior.
- Emergency production hotfixes or P1 incidents where immediate surgical remediation is required.
- Requirements have already been thoroughly discussed and an approved living spec draft exists.

---

## Operational Execution Runbook

### Phase 0: Classification

Before asking any question, classify the request and **announce the classification out loud** so the human can override:

- **Spike** — A feasibility question or exploratory investigation. Output is an answer or recommendation, not a spec. Example: "Can we integrate X library?" or "Is this approach viable?"
  - Process: Ask one clarifying question if needed, investigate, report recommendation. No spec, GATE 1 concludes with findings.

- **Bounded** — A well-scoped change to an existing flow already present in the codebase. A single new flag, a small endpoint adjustment, a one-file behavioral change.
  - Process: Ask 2–3 focused questions, present a short in-chat design summary, get approval, hand off to `jarn-spec` for a compact living spec update.

- **Architectural** — A new subsystem, a new capability, changes that restructure how components interact, or anything that introduces new API contracts or state machines.
  - Process: Full consultation and brainstorming runbook below.

**Classification rule**: When in doubt between two levels, take the heavier path. Hidden complexity discovered mid-consultation upgrades the classification — stop, announce the upgrade, and restart the appropriate path.

---

### Phase 1: Discover Intent & Brainstorm (Socratic Coaching)

Understand what the human is actually trying to accomplish before proposing anything.

- Read the request and available context (existing specs in `docs/specs/`, `CONTEXT.md`, `ARCHITECTURE.md`) to identify the intended outcome and who it serves.
- Ask **one focused question at a time**. Never dump multiple questions in a single message.
- Provide scaffolding when helpful: attach brief, concrete examples or best-practice options alongside open questions to guide developers who may not know the full technical landscape.
- Prioritize inquiry across these core dimensions:
  - **Purpose & Why**: Why is this needed? Who uses it? What problem does it solve?
  - **Viability & Value**: Is writing code the right solution, or can this be solved via configuration, existing tools, or process? Is it worth doing now, or should it be deferred or marked "won't do"?
  - **Constraints & Boundaries**: What must not change? What are the performance, security, data integrity, or compatibility boundaries?
  - **Edge Cases & Race Conditions**: What happens when input is invalid, concurrent requests arrive, or services fail?
  - **Success Criteria**: How will we empirically prove it works correctly?

- Write back a short understanding summary after the first round of answers. Separate confirmed facts from assumptions. Invite correction before treating it as the brief.
- If the request describes multiple independent subsystems, flag this immediately and help decompose into sub-tasks before proceeding. Each sub-task follows its own GATE 1 consultation cycle.

---

### Phase 2: Propose Options & Evaluate Trade-offs

Once intent and viability are confirmed, present implementation approaches — do not propose a single solution.

- Propose **2–3 distinct approaches** with concrete trade-offs for each.
- Lead with a **clear recommendation** and explain why it fits this context.
- Apply YAGNI ruthlessly: remove unnecessary features, abstractions, or premature optimizations from every option.
- Frame trade-offs in terms the human can evaluate: complexity, performance, maintainability, delivery speed.

Example structure:
```
Option A — [Name]: [One-line description]
  Trade-offs: [What you gain] vs [What you give up]

Option B — [Name]: [One-line description]
  Trade-offs: [What you gain] vs [What you give up]

Recommendation: Option A — because [specific reason tied to this project's context]
```

---

### Phase 3: Confirm Agreement

Before handing off, verify the agreed direction.

- Summarize the agreed intent, constraints, edge case handling, and chosen approach in 3–5 bullet points.
- Ask the human to confirm or correct. This is the last chance to adjust scope before spec writing begins.
- If the human requests changes, return to Phase 1 or Phase 2 as appropriate.

---

### Phase 4: Hand Off to jarn-spec

Once the human confirms the agreement:

- State clearly: "Consultation complete. Handing off to `jarn-spec` to synthesize the living specification in `docs/specs/`."
- Pass the agreed context — intent, constraints, chosen option, edge cases, and success criteria — as the input brief for `jarn-spec`.
- Do NOT write the spec yourself. Spec synthesis is the responsibility of the `jarn-spec` skill.
- Activate `jarn-spec` to produce or update `docs/specs/<feature-slug>.md` and the implementation plan.

---

## Red Flags

| Thought | Reality |
|---|---|
| "Requirements are clear enough, skip consultation" | Always classify first. Even a Spike needs a classification announcement. |
| "I'll ask all my questions at once to save time" | One question at a time. Batching overwhelms junior developers and produces shallow answers. |
| "I understand the request, I'll just propose a solution" | Probe intent first. Assumptions are the root cause of misaligned specs. |
| "This feels bounded, I'll skip the options step" | Bounded changes still need a short design presented for approval. |
| "The human approved the idea, so the spec is also approved" | Consultation approval permits spec drafting only. Spec approval is a separate hard stop in GATE 1. |
| "I'll write a quick spec draft during consultation to save time" | Spec synthesis belongs to jarn-spec. Keep concerns separated. |
| "The scope grew, but I'm almost done asking" | Hidden complexity upgrades the classification mid-consultation. Announce and adjust. |

---

## Consultation Output Checklist

Before handing off to `jarn-spec`, verify all items are confirmed:

- [ ] Classification announced and accepted (Spike / Bounded / Architectural)
- [ ] Purpose and intended outcome confirmed
- [ ] Viability confirmed (Proceed / Defer / Won't Do)
- [ ] Constraints and boundaries identified
- [ ] Key edge cases surfaced and agreed upon
- [ ] Success criteria defined
- [ ] 2–3 options proposed with trade-offs (Architectural path)
- [ ] Recommendation stated with reasoning
- [ ] Agreement summary confirmed by human
- [ ] Scope decomposed if multiple subsystems were identified
