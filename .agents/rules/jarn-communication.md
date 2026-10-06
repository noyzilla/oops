---
trigger: always_on
description: "Universal technical communication standard: Diátaxis framework, Caveman brevity, zero filler, exact codebase vocabulary, and technical payload preservation."
---

# Technical Communication Standard

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Add project-specific communication guidelines to your project's `AGENTS.md` or `docs/development/` only.

This document establishes the technical communication standard across all touchpoints: Chat, PR descriptions, Code Review comments, Markdown docs, Living Specs (`docs/specs/`), Commit messages, and Code Comments.

The goal is writing a tired engineer understands on first read.

## Core Rules

All written communication must strictly follow three principles:

- **Strict Word Economy (Cut Unused Words)**: Remove every filler word immediately.
  - Bad: "In order to initialize the database connection..."
  - Good: "To initialize the database connection..."
  - Bad: "It is important to note that this parameter is optional."
  - Good: "This parameter is optional."
- **Everyday Technical Verbs**: Use short, common verbs (`use` not `utilize`, `help` not `facilitate`, `run` not `execute`, `move` not `evacuate`).
- **Exact Codebase Vocabulary**: Use real symbol names, file paths, parameters, flags, or commands matching the codebase directly (`TimeoutSeconds`, `auth.go`, `--verbose`). Never invent abstract metaphors or synonyms.

## Payload Preservation Invariant

Compression applies to prose only. Technical payload must remain untouched:
- Never shorten, truncate, or abstract file paths, symbol names, shell commands, error tracebacks, line numbers, or code blocks.
- A dropped negation (`not`, `never`) breaks meaning. Preserve clarity over compression.

## Caveman Execution Pattern

During execution (GATE 2), adopt high-density Caveman phrasing:
- **Answer First Pattern**: `[thing] [action] [reason]. [next step]`
- **Kill Ceremony**: Zero greetings (`Sure!`), hedging (`basically`, `actually`), or closing fluff (`Hope this helps`).
- **Exception**: Suspend Caveman brevity during GATE 1 (Consultation & Trade-off Analysis) to evaluate architectural viability thoroughly.

## Diátaxis Framework (Document & Output Modes)

Adopt exactly **one** Diátaxis mode based on intent:

| Mode | Target & Purpose | Structural Rule |
| :--- | :--- | :--- |
| **How-to Guide** | Action + Work (Task Completion) | Step-by-step action items. Skip background context and theory. |
| **Reference** | Understanding + Work (Information Lookup) | Dry, precise, exhaustive, fact-only. Zero opinion or filler. |
| **Explanation** | Understanding + Learning (Architecture & Trade-offs) | Discusses design rationale, history, trade-offs. Opinions permitted exclusively here. |
| **Tutorial** | Action + Learning (Onboarding) | Guided learning with expected visual/terminal outputs at each milestone. |

### Mode Isolation Invariant
Never mix Diátaxis modes in a single output. Do not place theoretical explanations inside How-to guides or step-by-step guides inside Reference documents.

## Touchpoint Standards

- **Chat Dialogues**: High-density responses. Use **Explanation Mode** in GATE 1, **How-to Mode** or **Caveman Pattern** in GATE 2.
- **Living Specs & ADRs**: Specs (`docs/specs/`) strictly use **Reference Mode**. ADRs (`docs/adr/`) use **Explanation Mode**.
- **Commit Messages & PR Descriptions**: Use **How-to Mode** or **Reference Mode**. Focus strictly on *what changed* and *why*. Omit narrative filler ("I decided to fix...").
- **Code Review Comments**: State defects, edge-case failures, or rule violations directly with concrete examples or failing inputs. Omit subjective fluff ("This looks messy").
- **Inline Code Comments**: Use **Reference Mode** or **Explanation Mode**. Explain *why* an invariant exists, never narrate obvious syntax. Zero emojis, icons, or sequential step numbers (`// 1.`, `// 2.`).

## Rhythm & Sentence Structuring

- **Vary Sentence Length**: Mix short, impactful sentences for key invariants with longer sentences carrying single cohesive conditions.
- **One Thought per Sentence**: Split sentences carrying multiple distinct thoughts.
- **Concrete over Sterile**: State explicit failure modes and exact symbols (e.g., prefer "Renaming `UserTable` breaks the migration script" over "Schema alterations cause downstream issues").
