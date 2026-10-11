---
title: [How-to Guide / Workflow Title]
status: active
tags: [howto, workflow, template]
synapses: ["README.md"]
---

# How-to: [Guide / Workflow Title]

- **Type**: Setup Guide | Migration Runbook | Operational Playbook | Incident Guide | Troubleshooting Guide | Maintenance Procedure
- **Target Audience**: Developers, Operators, and AI Coding Agents

## Overview & Objective
Concise summary of what this procedure accomplishes, the problem it addresses, and the conditions under which it should be executed.

## Prerequisites & Dependencies
Mandatory toolchains, runtime versions, access permissions, or services required before execution:
- **Toolchain & Version**: Required CLI tools, package managers, and runtime engines.
- **Access & Permissions**: Required service credentials, IAM roles, or local privileges.
- **Environment & Secrets**: Required `.env` parameters or secret store keys.
- **Dependent Services**: Daemon processes, Docker containers, or database instances that must be running.

## Configuration & Environment Parity
Parameters and environment variables relevant to this workflow (aligned with `AGENTS.md` and `CONTEXT.md`):
- Parameter name, expected data type, default value, and configuration scope.

## Execution Procedure
Actionable commands and operational steps (use unnumbered bullets to prevent diff churn upon step insertion):
- **Pre-Flight Preparation**: Initial sanity checks, state validation, and backup routines before applying changes.
- **Primary Execution**: Step-by-step commands and operational tasks executed in top-to-bottom sequence.
- **Post-Execution Sync**: Cache clearing, daemon reloading, or configuration synchronizations.

## Verification & Health Check
Concrete evidence and verification commands to confirm successful execution (Exit Code 0):
- **Targeted Verification Command**: Project-native command or test suite validating the workflow.
- **Expected Result**: Specific terminal output, status code, log line, or health check payload confirming success.

## Rollback & Troubleshooting
Deterministic procedures to recover system stability upon failure:
- **Rollback Procedure**: Sequence of steps or scripts to revert mutations safely.
- **Common Failure Modes**: Known edge cases, error codes, and corresponding diagnostic steps.

---
[← Back to How-to Index](README.md)
