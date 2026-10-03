---
title: Webhook Daemon & Automated Deployment
status: active
tags: [webhook, server, docker, git-sync, image-recreation, auth]
synapses: ["ARCHITECTURE.md", "CONTEXT.md", "DESIGN.md", "docs/specs/cli.md"]
---

# Specification: Webhook Daemon & Automated Deployment

- **Status**: Active
- **Target Audience**: Developers, Operators, and AI Coding Agents
- **Parent Architecture**: [ARCHITECTURE.md](../../ARCHITECTURE.md)
- **Ubiquitous Language**: [CONTEXT.md](../../CONTEXT.md)
- **Companion Spec**: [docs/specs/cli.md](cli.md)

## Overview & Scope

Oops can run as a long-running background daemon (`oops server` or standalone container) listening for CI/CD deployment webhooks. It supports token-authenticated deployments for both pre-built Docker image pull/recreation and lightweight Git source checkouts into container volumes with optional pre/post build tool execution.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Server Mode**: Daemon process running the webhook HTTP server (`oops server`).
- **Action**: Deployment mechanism (`image` for container recreation, `git` for volume source sync).
- **Target Container**: Container authorized via `oops.enable=true` and matching secret token.

## Business Rules & Logic Invariants

### HTTP Daemon & Listening Configuration
- Listens on port defined by `OOPS_PORT` environment variable (default: `:8080`, fallback: `PORT`).
- Handlers:
  - `POST /update` (primary deployment endpoint)
  - `POST /deploy` (synonym endpoint for CI/CD parity)
  - `GET /healthz` (daemon liveness probe)

### Authentication Protocol
- Requests must supply a valid authentication token via:
  - Header: `X-Oops-Token: <TOKEN>` or `Authorization: Bearer <TOKEN>`
- Validation Logic:
  - Token is verified against each target container's `oops.secret` label.
  - If a global fallback secret is configured (`OOPS_SECRET`), it authorizes containers where `oops.secret` matches or is omitted.
  - Requests with missing or mismatched tokens are rejected immediately with `401 Unauthorized` before triggering any background operations.

### Target Validation & Discovery (`ValidateAndFindTargets`)
- Inspects all containers on the host Docker daemon matching `oops.enable=true`.
- Matches targets based on request parameters:
  - **Action Match**:
    - **Image Action (`action: "image"`)**: Matches containers whose current image name equals or contains `payload.image`.
    - **Git Action (`action: "git"`)**: Matches containers whose `oops.git.url` label matches `payload.url` (normalized). Requires non-empty `payload.tag`.
  - **Unified Target Selector (`payload.target`)**:
    - Optional selector supporting:
      - **Layer Target (`/<layer>`)**: Matches all containers in the layer (e.g. `/apps`).
      - **Scoped Service (`/<layer>/<service>`)**: Matches specific service in layer (e.g. `/apps/web`).
      - **Exact Name (`<service>`)**: Matches container with exact service/container name.
      - **Double Dot Wildcard (`<prefix>..`, `..<suffix>`, `..<keyword>..`)**: Matches containers matching the pattern (e.g. `app..`, `..worker`).
- If no matching authorized containers are found, responds with `404 Not Found`.

### Execution Modes & Sequential Protocol

#### 1. Image Recreation Mode (`action: "image"`)
- Asynchronously pulls the specified image (`docker pull <image>`).
- For each target container in sequence:
  - **Stop Hook**: If `oops.stop.cmd` is defined, executes the command inside the running container with timeout `oops.stop.timeout` (default 30s).
  - **Stop & Remove**: Stops container with 10s grace period and removes existing container.
  - **Recreate & Start**: Creates a new container preserving original network settings, host configuration, environment, mounts, and labels, then starts it.
  - **Inter-Service Delay Gap**: If `payload.delay` is specified, pauses for the designated duration before advancing to the next target container.
- **Image Pruning**: Automatically runs dangling image cleanup (`docker image prune -f`) to reclaim disk space.

#### 2. Git Sync Mode (`action: "git"`)
- For each target container in sequence:
  - Spawns a temporary lightweight container using `alpine/git` mounted via `VolumesFrom` targeting the target container.
  - Configures safe directory and executes `git fetch --all --tags --force && git checkout -f tags/<tag>` into directory `oops.git.dir`.
  - **Tool Helper Hook**: If `oops.tool.image` and `oops.tool.cmd` labels are present, runs a temporary helper container to execute migrations or assets builds.
  - **Stop Hook & Restart**: If `oops.stop.cmd` is defined, executes stop hook before restarting the target container.
  - **Inter-Service Delay Gap**: If `payload.delay` is specified, pauses before advancing to the next target container.

---

## Interface & Data Contracts

### Webhook Request Payload Schema

```json
{
  "action": "image",
  "target": "app..",
  "image": "ghcr.io/org/repo:latest",
  "url": "https://github.com/org/repo",
  "tag": "v1.2.0",
  "delay": "2s"
}
```

| Field | Type | Required | Description |
| :--- | :---: | :---: | :--- |
| `action` | `string` | No | `"image"` (default) or `"git"` |
| `target` | `string` | No | Target selector (`/apps`, `/apps/web`, `mysql`, `app..`, `..worker`) |
| `image` | `string` | Yes (for image) | Docker image to pull and deploy |
| `url` | `string` | Yes (for git) | Git repository URL matching `oops.git.url` |
| `tag` | `string` | Yes (for git) | Git tag to checkout (e.g., `"v1.0.0"`) |
| `delay` | `string` \| `int` | No | Inter-service pause between containers (e.g. `"2s"`, `5`) |

### HTTP Responses

| Status Code | Reason | Body Content |
| :---: | :--- | :--- |
| `200 OK` | Request validated & async worker launched | `{"status": "ok", "message": "Update triggered successfully"}` |
| `400 Bad Request` | Missing required fields or malformed JSON | Error message description |
| `401 Unauthorized` | Invalid or missing token header | `{"error": "Unauthorized"}` |
| `404 Not Found` | No matching running containers found | `{"error": "No matching targets found"}` |
| `405 Method Not Allowed` | HTTP method other than POST | `Method not allowed` |

---

## Dependency & Blast-Radius Matrix

- **Upstream Callers**: CI/CD Webhook callers (GitHub Actions, GitLab CI, curl).
- **Downstream Dependencies**: Host Docker Engine API (`/var/run/docker.sock`), `alpine/git` image.
- **Bounded Blast Radius**:
  - `cmd/server.go`: Subcommand initialization for webhook daemon.
  - `internal/config/`: Port and secret configuration.
  - `internal/webhook/`: HTTP handler, payload parser, router.
  - `internal/docker/`: Docker SDK client, exec stop hooks, container recreation, git pull runner.

---

## Verification & Acceptance Criteria

- **Targeted Test Command**: `go test -v -race ./internal/webhook/... ./internal/docker/...`
- **Acceptance Scenarios**:
  - `POST /update` with invalid token returns `401 Unauthorized`.
  - `POST /update` with missing image on image action returns `400 Bad Request`.
  - `POST /update` matching target container triggers stop hook and recreates container.
  - `POST /update` with git action executes temporary git checkout and restarts target container.
