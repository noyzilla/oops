# Naming Conventions

> **Do not modify this file.** It is part of the Jarn framework and will be overwritten during framework updates (`jarn-framework-update` skill). Add project-specific naming conventions to your project's `CONTEXT.md` only.

This document defines the naming conventions for configurations, variables, slugs, and domains across the project.

## Configuration Naming Principles

### Nested Hierarchy as Path Notation
- **Rule:** Nested configuration objects must act as a "Path" indicating the category, never constructed as a natural language sentence.
- **Rationale:** Path notation facilitates easy searching (grep/IntelliSense), keeps the codebase organized, and groups configurations logically.
- **Bad:** `CONFIG.WORKER.RUN_CLEANUP_EVERY_MS` (Constructed as a sentence)
- **Good:** `CONFIG.WORKER.CLEANUP_INTERVAL_MS` (Acts as a path)

### Noun to Modifier to Unit Pattern
- **Rule:** Key names must follow the strict order of **[Topic (Noun)] -> [Property/State (Modifier)] -> [Unit (if applicable)]**.
- **Rationale:** Ensures related variables are grouped together alphabetically and eliminates ambiguity regarding units of measurement.
- **Bad:** `MAX_TIMEOUT_MS` (Modifier precedes the noun)
- **Good:** `TIMEOUT_MAX_MS` (Topic: Timeout, Modifier: Max, Unit: MS)
- **Good:** `INACTIVITY_DAYS`, `RETRY_LIMIT_COUNT`

### Environment Variable Parity
- **Rule:** Variables in `.env` files (which are flat) must explicitly reflect the nested object structure in the code by using prefixes.
- **Rationale:** Allows developers to instantly map where an environment variable is injected within the system architecture.
- **Example:** `DATABASE_POOL_SIZE` in `.env` strictly maps to `CONFIG.DATABASE.POOL_SIZE`.

### Compound Words as Single Identity
- **Rule:** Compound words or single-entity concepts within the system's context must NOT be separated by underscores (`_`), even if written separately in natural language.
- **Rationale:** Reduces verbosity and preserves the concept as a single logical entity rather than a noun with a modifier.
- **Bad:** `AUTO_START_DELAY_MS` (Makes "Auto" look like a modifier for "Start")
- **Good:** `AUTOSTART_DELAY_MS` ("Autostart" is treated as a single noun)
- **Good:** `WEBSOCKET_PORT` (Not `WEB_SOCKET_PORT`), `FILENAME` (Not `FILE_NAME`)

## Slug & Document Path Notation

### Path Notation over Sentence-Like Slugs
- **Rule:** Slugs for files (`docs/**`), ADRs, living specs, runbooks, and Git branches must use `kebab-case` structured as a hierarchical path: **`[Domain/Topic]-[Modifier/Subtopic]-[Detail/Entity]`**. Never construct slugs as natural language sentences.
- **Rationale:** Groups related files alphabetically, enables instant pattern searches (`grep`, glob), and eliminates deeply nested folder structures while preserving structural depth for AI context ingestion.
- **Bad:** `how-to-login-with-google.md`, `fix-the-broken-database-timeout.md`
- **Good:** `auth-oauth-google.md`, `db-timeout-retry.md`

### Slug Formatting Invariants
- **Lowercase & Hyphens Only**: Strictly lowercase `a-z`, digits `0-9`, and single hyphens `-`. No uppercase, spaces, or underscores.
- **Concise Scope**: 2–5 words focusing on domain intent. Omit conversational filler words (`the`, `a`, `and`, `how-to`).
- **Numeric Prefixes**: When sequential order or identifier tracking is required (ADRs in `docs/decisions/` and runbooks in `docs/development/`), use a 4-digit zero-padded prefix: `XXXX-<slug>.md` (e.g., `0001-project-identity.md`).

## Domain & Intent-Based Naming

- **Domain Alignment**: All new or modified entity names, database columns, and API parameters match the ubiquitous language defined in `CONTEXT.md`.
- **Intent-Based Naming**: Variables and functions reflect domain intent, not mechanism. Generic placeholders (`data`, `temp`, `helper`, `manager`, `process`) are avoided.
- **Boolean Predicates**: Boolean variables and functions use clear prefixes (`is_active`, `has_access`, `can_modify`, `should_retry`).
