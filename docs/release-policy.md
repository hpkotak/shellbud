# Release Policy

This repository uses a risk-based review model with hard CI gates.

## Required CI Checks

The following checks must pass before merge:

- `format` (Go formatting check)
- `test` (`go vet`, `go test`, `go test -race`)
- `lint` (`golangci-lint`)
- `coverage` (threshold gate in `scripts/check_coverage.sh`)

## Coverage Thresholds

Current required thresholds:

- Total coverage: `>= 85%`
- Critical packages: `>= 90%`
  - `./cmd`
  - `./internal/repl`
  - `./internal/provider`
  - `./internal/safety`
  - `./internal/prompt`
- Temporary floor: `./internal/setup >= 70%`

## Review Requirements

- High-risk runtime paths are CODEOWNED in `.github/CODEOWNERS`.
- Enable GitHub branch protection / rulesets with:
  - Required status checks (the CI jobs above)
  - Require review from Code Owners

This allows low-risk changes (for example docs-only changes) to auto-merge when checks pass, while keeping human review on high-risk paths.
