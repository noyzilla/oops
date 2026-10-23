# System Architecture

## Overview
This document provides the high-level architecture, module boundaries, and design principles of the system.

## Core Architectural Pillars
- **1:1 Dev-to-Prod Parity**: The identical turnkey infrastructure blueprint (`oopsbox/`) runs across local workstations (macOS / Linux) and production servers, standardizing network isolation (`oops_default`, `oops_edge`, `oops_db`, `oops_storage`), storage hierarchy (`data/`, `backups/`), and DNS resolution to eliminate environment drift.
- **Git-Driven Infrastructure & Capsule Streaming (No SSH Snowflakes)**: Git and local remote capsules (`/@<remote>/`) serve as the authoritative source of truth for all stack definitions (`compose.<stack>.yaml`) and modular service configs (`config/<service>/`). Direct SSH streaming (`oops deploy`) and webhook reconciliation eliminate manual in-place server editing over SSH.
- **Effortless Tech Stack Upgrades**: Decoupled pure stack topologies (`edge`, `db`, `storage`, `tools`, `apps`) controlled via `x-oops-stack:` allow independent runtime upgrades with zero blast-radius on adjacent services, backed by safe sequential rolling updates and pre-stop lifecycle hooks.

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
- [Living Specifications](docs/specs/): [CLI Orchestration](docs/specs/cli.md), [Oopsbox Workstation](docs/specs/oopsbox.md), [Remote Server Management](docs/specs/remote.md), [IAP SSH Tunnel](docs/specs/tunnel.md), [Native DNS Daemon](docs/specs/dns.md), [Webhook Daemon](docs/specs/webhook.md), [Volume Guard](docs/specs/volume-guard.md) (`docs/specs/`) - Feature and subsystem contracts combining domain rules, API schemas, and dependency blast-radius matrices (template: [.agents/templates/docs/spec.md](.agents/templates/docs/spec.md)).
- **Architectural Decisions** (`docs/adr/`) - Strategic architectural decision records (ADR) with explicit `.deprecated.md` and `.superseded.md` lifecycle naming (template: [.agents/templates/docs/adr.md](.agents/templates/docs/adr.md)).
- **Architecture Deep-Dives** (`docs/architecture/`) - Subsystem topologies, component interaction diagrams, and system data flows, including [Persistent Data and Storage Path Abstraction](docs/architecture/data-persistent-storage.md) (template: [.agents/templates/docs/architecture.md](.agents/templates/docs/architecture.md)).
- **Design Specifications** (`docs/design/`) - Reusable component tokens, form styling, and accessibility standards (template: [.agents/templates/docs/design.md](.agents/templates/docs/design.md)).
- **How-to Guides & Operations Wiki** (`docs/howto/`) - Developer onboarding, local environment setup, deployment and operational playbooks, migration runbooks, troubleshooting, and maintenance procedures (template: [.agents/templates/docs/howto.md](.agents/templates/docs/howto.md)).
