# How-to Guides & Operations Wiki

Comprehensive operational manuals, procedural how-to guides, and runbooks across development, operations, and maintenance.

## Navigation & Guidelines
- Each guide is a focused, actionable procedure detailing exact prerequisites, commands, expected verification outputs, and rollback steps.
- For isolated topics (1–2 guides), place files directly in this directory as `<slug>.md`.
- When a topic expands (>= 3 guides), create a sub-directory `docs/howto/<subtopic>/` containing its own `README.md` as a sub-topic table of contents.

## Table of Contents

### Getting Started & Local Environment
- [Local Development Setup](local-setup.md) — Toolchain installation, environment setup, and dependency management.

### Database & Migrations
- [Database Migration & Rollback](db-migration.md) — Running schema migrations, inspecting database state, and reverting failed changes.

### Deployment & Environments
- [Deployment Playbook](deployment.md) — Staging and production deployment procedures, smoke tests, and rollback routines.

### Diagnostics & Incidents
- [Incident Response & Troubleshooting](incident-response.md) — Triage workflows, log collection, and diagnostic procedures.
