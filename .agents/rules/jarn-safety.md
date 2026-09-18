# Safety Boundaries & Non-Negotiable Invariants

This document establishes the safety invariants and escalation gates for all contributors (human engineers and AI agents) operating within this codebase.

## Non-Negotiable Rules

The following actions are strictly prohibited without prior explicit human confirmation:

- **Database Destruction**: Dropping databases, schemas, or tables, executing table truncation, or applying unverified data-destructive migrations.
- **Git History Rewrite**: Force-pushing (`git push --force` or `--force-with-lease`) to remote branches, deleting remote branches, or hard-resetting shared branches.
- **Direct Edits & Commits to Main (Step 0 Invariant)**: Modifying, creating, or committing files directly on the `main` or production branch. Before making any codebase changes, contributors and agents MUST verify `git branch --show-current` and branch out (`git checkout -b <type>/<slug>`).
- **Credential Exposure**: Adding, modifying, reading, or printing production secrets, private keys, authentication tokens, API keys, or `.env` files containing sensitive credentials.
- **Uncontrolled Dependencies**: Introducing new third-party libraries, packages, or external dependencies that have not been explicitly discussed and agreed upon.
- **Unbounded Deletion**: Recursively deleting directories or bulk deleting source files outside of designated build output or scratch folders.

## Stop and Ask Escalation Gates

Contributors and agents must pause execution and consult when any of the following conditions arise:

- **Ambiguous Requirements**: The request lacks clear acceptance criteria or presents multiple conflicting implementation paths.
- **Architectural Deviation**: An intended change conflicts with patterns established in `ARCHITECTURE.md` or active Living Specifications (`docs/specs/`).
- **Unforeseen Impact**: Modifying a module introduces cascading errors or breaks contracts across dependent modules.
- **Scope Expansion**: The implementation requires touching files or services beyond the boundaries of the approved plan.
