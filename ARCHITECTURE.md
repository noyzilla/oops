# System Architecture

## Overview
This document provides the high-level architecture, module boundaries, and design principles of the system.

## Core Architectural Pillars
- **1:1 Dev-to-Prod Parity**: The identical turnkey infrastructure blueprint (`oopsbox/`) runs across local workstations (macOS / Linux) and production servers, standardizing network isolation (`net-edge`, `net-db`), storage hierarchy (`data/`, `backups/`), and DNS resolution to eliminate environment drift.
- **Git-Driven Infrastructure as Code (No SSH Snowflakes)**: Git serves as the single authoritative source of truth for all stack definitions and modular service configs (`stacks/<group>/<service>/`). Automated webhook reconciliation eliminates manual in-place server editing over SSH.
- **Effortless Tech Stack Upgrades**: Decoupled multi-group topologies (`edge`, `db`, `tool`, `apps`) allow independent runtime upgrades with zero blast-radius on adjacent services, backed by safe sequential rolling updates and pre-stop lifecycle hooks.

## Architectural Principles
- **Contract-First Design**: Define interfaces, data schemas, and API contracts before implementation.
- **Linear Version Lifecycle**: Development proceeds forward on a single canonical line (`main`). A new release supersedes the previous release; parallel version branches are not maintained unless explicitly required by an external compatibility obligation.
- **Minimizing Concurrent State**: Limit work-in-progress by executing tasks serially by default. Concurrency is permitted strictly when proven orthogonal via the Dependency & Blast-Radius Matrix.
- **Separation of Concerns**: Isolate domain logic, operational orchestration, and external I/O into modular components.
- **Explicit Over Implicit**: Favor clear, observable code structures over hidden side effects or implicit magic.
- **Progressive Disclosure**: Keep high-level maps here at the root; extract deep-dive specifications into `docs/architecture/`.

## Core Modules
- **Core Engine**: Implements the primary business logic and domain entities.
- **Interface Adapters**: Translates external requests (CLI, HTTP, RPC) into internal domain operations.
- **Data & Persistence**: Manages storage abstractions and external service integrations.

## System Documentation & Deep-Dives
In accordance with the Mirror Index Pattern and system documentation taxonomy in [docs/README.md](docs/README.md):
- [Living Specifications](docs/specs/): [CLI Orchestration](docs/specs/cli.md), [Oopsbox Workstation](docs/specs/oopsbox.md), [Native DNS Daemon](docs/specs/dns.md), [Webhook Daemon](docs/specs/webhook.md), [Storage Guard](docs/specs/storage-guard.md) (`docs/specs/`) - Feature and subsystem contracts combining domain rules, API schemas, and dependency blast-radius matrices (template: [.agents/templates/docs/spec.md](.agents/templates/docs/spec.md)).
- **Architectural Decisions** (`docs/adr/`) - Strategic architectural decision records (ADR) with explicit `.deprecated.md` and `.superseded.md` lifecycle naming (template: [.agents/templates/docs/adr.md](.agents/templates/docs/adr.md)).
- **Architecture Deep-Dives** (`docs/architecture/`) - Subsystem topologies, component interaction diagrams, and system data flows, including [Persistent Data and Storage Path Abstraction](docs/architecture/data-persistent-storage.md) (template: [.agents/templates/docs/architecture.md](.agents/templates/docs/architecture.md)).
- **Design Specifications** (`docs/design/`) - Reusable component tokens, form styling, and accessibility standards (template: [.agents/templates/docs/design.md](.agents/templates/docs/design.md)).
- **Development Workflows** (`docs/development/`) - Developer onboarding, local environment setup, and migration runbooks (template: [.agents/templates/docs/development.md](.agents/templates/docs/development.md)).
