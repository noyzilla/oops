# Contributing & Development Guide

Thank you for contributing to **Oops**! This document outlines the development workflow, testing standards, commit conventions, and release procedures.

---

## Local Development

### Prerequisites
- [Go 1.22+](https://golang.org/)
- [Docker](https://www.docker.com/)

### Running Tests
Execute all unit tests across modules:
```bash
go test -v ./...
```

### Building the Binary
```bash
go build -o oops .
```

---

## Commit Conventions

This project strictly follows the [Conventional Commits](https://www.conventionalcommits.org/) specification coupled with [Semantic Versioning (SemVer)](https://semver.org/).

### Format
```text
<type>(<scope>): <subject>
```

### Commit Types & SemVer Impact

| Prefix | Description | SemVer Bump | Example |
| :--- | :--- | :---: | :--- |
| **`feat:`** | Introduces a new feature | **MINOR** (`v0.1.0` -> `v0.2.0`) | `feat: add oops.git.url matching` |
| **`fix:`** | Fixes a bug | **PATCH** (`v0.1.0` -> `v0.1.1`) | `fix: handle missing secret label` |
| **`feat!:`** or `BREAKING CHANGE:` | Introduces breaking changes | **MAJOR** (`v0.1.0` -> `v1.0.0`) | `feat!: enforce url field in git mode` |
| **`docs:`** | Documentation changes only | None / Patch | `docs: update deployment examples` |
| **`refactor:`** | Code refactoring without feature/fix | None / Patch | `refactor: simplify target validation` |
| **`test:`** | Adding or updating tests | None | `test: add git url normalization test` |
| **`chore:`** | Maintenance tasks or tooling updates | None | `chore: clean build scripts` |

---

## Releasing & Tagging with `svu`

We use [svu (Semantic Version Util)](https://github.com/caarlos0/svu) — a pure Go tool that analyzes Conventional Commits since the last Git tag to calculate the next SemVer version.

### Installation
```bash
go install github.com/caarlos0/svu@latest
# or via Homebrew:
# brew install caarlos0/tap/svu
```

### Tagging and Releasing
```bash
# Check current tag
svu current

# Preview the next calculated version
svu next

# Create and push the new Git tag
git tag $(svu next) && git push origin $(svu next)
```
