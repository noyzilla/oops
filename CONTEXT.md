# Domain Context & Ubiquitous Language

This document establishes the official domain glossary and ubiquitous language for this project. Both human engineers and AI coding assistants must adhere to these definitions across source code, database schemas, API contracts, tests, and documentation.

## Purpose & Authority

- **Eliminate Naming Drift**: Prevent AI agents and contributors from introducing conflicting terms or synonyms for the same domain entity.
- **Consistent Code Identifiers**: Variable names, function identifiers, database columns, and API parameters must strictly mirror the terms defined here.
- **Single Source of Truth**: When domain requirements evolve, update this document before refactoring codebase identifiers.

## Domain Glossary Format

Every domain concept is documented using the following structure:

- **Term Name**: The exact canonical noun or verb phrase representing the entity or operation.
- **Definition**: The precise functional meaning, boundary, and lifecycle of the entity within this system.
- **Permitted Synonyms**: Rare, approved alternate contexts where a variant may appear (e.g., UI labels versus database columns).
- **Avoid (Prohibited Synonyms)**: Ambiguous, deprecated, or misleading terms that must never be used in code, APIs, or schemas.

---

## Core Domain Entities

*(Define your project-specific domain entities here following the format above. For example:)*

### Entity: User
- **Canonical Identifier**: `user_id`
- **Definition**: An authenticated human identity holding login credentials and account ownership.
- **Permitted Synonyms**: `account_owner` (in billing context only)
- **Avoid**: `member`, `client`, `person`, `actor`, `profile`

---

## Maintenance Guidelines

- **Consult Before Naming**: When creating a new model, database table, or API endpoint, verify whether a matching concept already exists in this glossary.
- **Flag Unknown Terms**: If a new requirement introduces a concept not listed here, define it in `CONTEXT.md` during the implementation planning phase.
- **Enforce Code Reviews**: Reviewers and automated review checks must reject PRs introducing terms listed under the Avoid sections.
