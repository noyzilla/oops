# Naming Conventions [หลักการตั้งชื่อ]

This document defines the naming conventions for configurations, variables, and domains across the project.

## Configuration Naming Principles [หลักการตั้งชื่อการตั้งค่า]

### Nested Hierarchy as Path Notation [โครงสร้างคือเส้นทาง ไม่ใช่ประโยค]
- **Rule:** Nested configuration objects must act as a "Path" indicating the category, never constructed as a natural language sentence.
- **Rationale:** Path notation facilitates easy searching (grep/IntelliSense), keeps the codebase organized, and groups configurations logically.
- **Bad:** `CONFIG.WORKER.RUN_CLEANUP_EVERY_MS` (Constructed as a sentence)
- **Good:** `CONFIG.WORKER.CLEANUP_INTERVAL_MS` (Acts as a path)

### Noun to Modifier to Unit Pattern [คำนามหลักต้องมาก่อนเสมอ]
- **Rule:** Key names must follow the strict order of **[Topic (Noun)] -> [Property/State (Modifier)] -> [Unit (if applicable)]**.
- **Rationale:** Ensures related variables are grouped together alphabetically and eliminates ambiguity regarding units of measurement.
- **Bad:** `MAX_TIMEOUT_MS` (Modifier precedes the noun)
- **Good:** `TIMEOUT_MAX_MS` (Topic: Timeout, Modifier: Max, Unit: MS)
- **Good:** `INACTIVITY_DAYS`, `RETRY_LIMIT_COUNT`

### Environment Variable Parity [การแมป .env เข้ากับ Hierarchy อย่างเป็นระบบ]
- **Rule:** Variables in `.env` files (which are flat) must explicitly reflect the nested object structure in the code by using prefixes.
- **Rationale:** Allows developers to instantly map where an environment variable is injected within the system architecture.
- **Example:** `DATABASE_POOL_SIZE` in `.env` strictly maps to `CONFIG.DATABASE.POOL_SIZE`.

### Compound Words as Single Identity [รวมคำที่เป็นความหมายเดียวกัน ห้ามใช้ Underscore คั่น]
- **Rule:** Compound words or single-entity concepts within the system's context must NOT be separated by underscores (`_`), even if written separately in natural language.
- **Rationale:** Reduces verbosity and preserves the concept as a single logical entity rather than a noun with a modifier.
- **Bad:** `AUTO_START_DELAY_MS` (Makes "Auto" look like a modifier for "Start")
- **Good:** `AUTOSTART_DELAY_MS` ("Autostart" is treated as a single noun)
- **Good:** `WEBSOCKET_PORT` (Not `WEB_SOCKET_PORT`), `FILENAME` (Not `FILE_NAME`)

## Domain & Intent-Based Naming

- **Domain Alignment**: All new or modified entity names, database columns, and API parameters match the ubiquitous language defined in `CONTEXT.md`.
- **Intent-Based Naming**: Variables and functions reflect domain intent, not mechanism. Generic placeholders (`data`, `temp`, `helper`, `manager`, `process`) are avoided.
- **Boolean Predicates**: Boolean variables and functions use clear prefixes (`is_active`, `has_access`, `can_modify`, `should_retry`).
