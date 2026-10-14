# Task Tracking & Active Iteration

This file tracks the active focus, immediate next actions, and backlog deliverables for the repository.

## Active Focus
- Remote Delivery Lifecycle & Symmetric Remote Model (`feat/remote-delivery-lifecycle`):
  - Milestone 1: Exploration `exp-2610-1` and Blueprint Templates (`compose.yaml.example`, `oops.yaml.example`, `.oopsallow.example`).
  - Milestone 2: `.oopsallow` default-deny whitelist engine in `internal/sync/allow.go`.
  - Milestone 3: Blueprint initialization during `oops remote connect` in `internal/remote/blueprint.go`.
  - Milestone 4: Services Dependency Resolution Algorithm in `internal/remote/resolve.go`.
  - Milestone 5: Symmetric Git/Remote commands (`oops remote push` and `oops remote pull`), `oops deploy` alias with pure delivery semantics.
  - Milestone 6: Deprecation and complete removal of legacy standalone `oops sync`.
  - Milestone 7: Living specifications (`remote.md`, `cli.md`, `env-sync.md`), tests, and CHANGELOG synchronization.

## Immediate Next Actions
- Review uncommitted diff and prepare for GATE 3 Pre-Merge Audit.
