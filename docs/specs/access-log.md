---
title: Edge Proxy Access Log Stream & Filtering Engine
status: active
tags: [cli, logs, proxy, caddy, nginx, access-log, filter]
synapses: ["CONTEXT.md", "docs/specs/cli.md"]
---

# Specification: Edge Proxy Access Log Stream & Filtering Engine

- **Last Verified**: 2026-11-02
- **Target Audience**: Developers and AI Coding Agents

## Overview & Scope

The `oops access-log` command provides real-time streaming, normalization, and regex-powered filtering of HTTP access logs from Oopsbox reverse proxy containers (e.g. Caddy, Nginx). It converts raw, unstructured, or JSON log payloads into an ergonomic, single-line human-readable terminal table while offering precise filtering across status, method, host, path, scheme, client IP, and latency bounds.

## Domain Context & Ubiquitous Language

Terms strictly follow [CONTEXT.md](../../CONTEXT.md):
- **Access Log**: An individual HTTP transaction record capturing incoming client requests and proxy responses (method, status, host, path, latency).
- **System Log**: Internal proxy runtime messages (e.g. TLS certificate renewals, worker reload events, DNS errors) produced by the reverse proxy daemon.
- **Edge Proxy**: The ingress reverse proxy container running in the Oopsbox environment (conventionally `caddy-proxy` or services residing in stack `/edge`).
- **Decoder**: Format-specific parser responsible for decoding raw container log streams into normalized `AccessRecord` structs.
- **Forbidden Synonym**: `edge-log` (dropped in favor of `access-log`), `web-log`, `traffic-dump`.

## Business Rules & Logic Invariants

### Command Interface & Default Streaming Invariant
- **Command Signature**: `oops access-log [container]`
- **Target Discovery Hierarchy**:
  1. If `[container]` argument is provided, target that container directly.
  2. If omitted, inspect active Docker containers for `caddy-proxy` or any container belonging to the `/edge` stack.
  3. Fallback: Search for any running container whose name contains `caddy` or `nginx`.
- **Default Follow Mode**: `-f` / `--follow` defaults to `true` (continuous streaming). Passing `-d` / `--no-follow` sets follow to `false` (dump existing buffer and exit).
- **Default Buffer Depth**: `-t` / `--tail` defaults to `100` historical lines.

### Log Classification & Visibility Filtering
- **HTTP Access Logs Only by Default**: Internal proxy system logs (TLS renew, cache swap, debug messages) are suppressed by default to prevent visual noise.
- **`--all` Flag**: When `--all` is passed, system logs are rendered with a distinct `[SYS]` marker and simplified format.

### 1:1 Column-to-Flag Filter Invariants
All filtering flags map 1:1 to visible table columns and support standard regular expressions:
- `--level`: Regex pattern matching log level (`INFO`, `WARN`, `ERROR`, `DEBUG`).
- `--status`: Regex pattern matching HTTP status code string (`200`, `^5..`, `404|502`).
- `--method`: Regex pattern matching HTTP method (`GET`, `POST`, `OPTIONS`).
- `--host` (alias `--domain`): Regex pattern matching HTTP request `Host`.
- `--path`: Regex pattern matching request URI/path (`^/api/v1/.*`).
- `--scheme`: Regex pattern matching URL scheme (`https?`).
- `--client` (alias `--ip`): Regex pattern matching client remote IP address.

### Latency Comparison Filter Invariant
The `--latency` flag parses comparison operators and duration units:
- Operators: `>`, `>=`, `<`, `<=`, or range `>10ms,<100ms` / `10ms..100ms`.
- Units: standard Go durations (`ms`, `s`, `us`, `µs`).
- Bare numbers without units default to milliseconds (e.g. `>50` = `>50ms`).

### Terminal Hygiene & Non-TTY Invariant
- Single-line compact table layout:
  `TIME      LEVEL  STATUS METHOD  HOST            PATH                LATENCY  CLIENT`
- ANSI status color coding:
  - 2xx: Green
  - 3xx: Cyan
  - 4xx: Yellow
  - 5xx: Red / Bold
- Latency alert colors: `<50ms` (normal), `50ms-300ms` (yellow), `>300ms` (red).
- When output is not a terminal (e.g. piped to `grep` or redirected to file), ANSI escape codes are automatically stripped.

## Flow & State Machine

```
[Start oops access-log]
         │
         ▼
[Resolve Target Container] ──(Not Found)──> [Exit with Error]
         │
         ▼
[Detect Proxy Type (Caddy/Nginx)]
         │
         ▼
[Open Docker Logs Stream (--tail 100, follow=true)]
         │
         ▼
┌─ [Stream Reader Loop] ◄────────────────────────────────┐
│        │                                               │
│        ▼                                               │
│ [Decoder.Decode(line)]                                 │
│        │                                               │
│        ├─ (System Log && !IncludeSystem) ──────────────┤ (Drop)
│        │                                               │
│        ▼                                               │
│ [Apply Filter Criteria (Regexes + Latency)]            │
│        │                                               │
│        ├─ (Does Not Match Filter) ─────────────────────┤ (Drop)
│        │                                               │
│        ▼                                               │
│ [ANSI Formatter.Render(record)] ───────────────────────┘
│
▼
[Stream Terminated / SIGINT] ──> [Clean Exit 0]
```

## Interface & Data Contracts

### Normalized Models (`internal/accesslog/types.go`)

```go
type AccessRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	Level     string        `json:"level"`
	Status    int           `json:"status"`
	Method    string        `json:"method"`
	Host      string        `json:"host"`
	Path      string        `json:"path"`
	Scheme    string        `json:"scheme"`
	Latency   time.Duration `json:"latency"`
	ClientIP  string        `json:"client_ip"`
	IsSystem  bool          `json:"is_system"`
	Message   string        `json:"message"`
	Raw       []byte        `json:"raw"`
}

type LatencyFilter struct {
	Min      *time.Duration
	Max      *time.Duration
	MinEqual bool
	MaxEqual bool
}

type FilterCriteria struct {
	LevelPattern  *regexp.Regexp
	StatusPattern *regexp.Regexp
	MethodPattern *regexp.Regexp
	HostPattern   *regexp.Regexp
	PathPattern   *regexp.Regexp
	SchemePattern *regexp.Regexp
	ClientPattern *regexp.Regexp
	Latency       *LatencyFilter
	IncludeSystem bool
}

type Decoder interface {
	Decode(line []byte) (*AccessRecord, error)
}
```

## Dependency & Blast-Radius Matrix

- **Upstream Callers (Inbound)**: `cmd/access_log.go`, `cmd/root.go`.
- **Downstream Dependencies (Outbound)**: `internal/docker` (container discovery & logs stream), standard Go `regexp`, `time`, `encoding/json`.
- **Bounded Blast Radius**:
  - `docs/specs/access-log.md`
  - `internal/accesslog/*`
  - `cmd/access_log.go`
  - `cmd/access_log_test.go`
  - `cmd/root.go`

## Verification & Acceptance Criteria

- **Targeted Test Command**:
  `go test ./internal/accesslog/... ./cmd/... -run "TestAccessLog.*"`
- **Acceptance Scenario - Caddy JSON Parsing**: Correctly parses Zap JSON access logs into `AccessRecord` with accurate latency calculation and status code.
- **Acceptance Scenario - Filter Matching**: Regex patterns on status, method, host, path, and comparison operators on latency accurately filter matched records and discard non-matching lines.
- **Acceptance Scenario - Auto-Follow & Tail Defaults**: Command defaults to follow mode (`-f=true`) and 100 lines tail (`-t=100`).

