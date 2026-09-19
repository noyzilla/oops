# CLI Output & Logging Guidelines

This document governs the formatting, structure, and standards for terminal outputs, application logs, and CLI responses across the **oops** project. Since this is a backend webhook handler without a graphical UI, this document serves as the "Design System" for our console output.

## Core Aesthetic & Philosophy
- **Clean & Focused**: Logs must be highly readable, avoiding visual noise or unnecessary decorations.
- **Machine-Parsable**: Critical logs should be structured (JSON) for easy consumption by log aggregators.
- **Actionable**: Every error log must provide enough context to diagnose the issue without guessing.

## Application Logging Standards (Backend / Webhook)

### Log Levels
- **INFO**: General operational events (e.g., `Webhook received: {id}`, `Server started on :8080`).
- **DEBUG**: Verbose trace information for local development (e.g., `Payload parsed successfully`, `DB connection pool stat`).
- **WARN**: Unexpected situations that do not halt the main execution but require attention.
- **ERROR**: Operations that failed, halting a specific request or webhook processing. Must include stack trace or deep context.
- **FATAL**: Critical failures requiring immediate application shutdown (e.g., `Failed to connect to database on startup`).

### Structured Logging (Production)
In production environments, logs should be output in JSON format.
**Example Structure**:
```json
{
  "timestamp": "2026-09-19T23:00:00Z",
  "level": "INFO",
  "service": "oops-webhook",
  "message": "Webhook processed successfully",
  "event_id": "req-12345",
  "latency_ms": 45
}
```

## Local Development & CLI Output

For local development or CLI commands, human-readable text output is preferred over JSON.

- **Success Messages**: Printed in neutral white or green.
- **Errors**: Printed to `stderr` in red, containing the `[ERROR]` prefix.
- **Context Variables**: Highlighted or wrapped in quotes for readability (e.g., `Processing event ID 'req-12345'`).

## Maintenance Guidelines
Any changes to the logging framework, log formatting rules, or standardized tags must be documented here.
