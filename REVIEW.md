# Code Review & Pre-Merge Checklist [รายการตรวจทานโค้ดก่อนรวมงาน]

This document provides the standard pre-merge checklist for contributors and reviewers (human engineers and AI agents) before merging pull requests.

It inherits the **Universal Code Review & Pre-Merge Standard** from [.agents/rules/jarn-review.md](.agents/rules/jarn-review.md). Universal quality gates are synchronized automatically across projects via `./.agents/scripts/jarn-update.sh`.

## How to Use This Checklist [วิธีใช้รายการตรวจทาน]

- Review the diff against every applicable section below.
- Check every matching row before approving or merging.
- Any deliberate deviation or deferred check must be justified explicitly in the pull request description.

---

## Universal Quality Gates (Inherited from .agents/rules/jarn-review.md)

Please review and check all items defined in the Universal Quality Gates: [.agents/rules/jarn-review.md](.agents/rules/jarn-review.md).

---

## Project-Specific Review Extensions [การส่วนขยายเฉพาะโปรเจกต์]

Add custom review gates, domain compliance checks, performance budgets, or security audits specific to this project below:

- [ ] **UI Visual Proof (When Applicable)**: For user-visible UI changes, before and after screenshots or recordings are attached in the PR description, complying with tokens in `DESIGN.md`.
- [ ] **Domain Invariants**: Business constraints and state transitions match the subsystem specification in `docs/specs/`.
