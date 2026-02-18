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

## Release Workflow

Releases are published via [goreleaser](https://goreleaser.com) + GitHub Actions.

### How to release

```bash
git tag v0.2.0
git push origin v0.2.0
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which:

1. Runs `make validate` (same CI gates as PRs)
2. Builds `afm-bridge` on a macOS arm64 runner (Xcode 26), packages as tarball with SHA256 checksum
3. Builds cross-platform Go binaries named `sb` (darwin/linux, amd64/arm64)
4. Creates a GitHub release with archives, checksums, and the `afm-bridge` tarball
5. Pushes a Homebrew formula to `hpkotak/homebrew-tap`

### Distribution

- **Homebrew:** `brew install hpkotak/tap/sb` — on macOS Apple Silicon, the formula also installs `afm-bridge` for Apple Foundation Models support
- **GitHub Releases:** download from https://github.com/hpkotak/shellbud/releases (includes `afm-bridge` tarball for macOS arm64)
- **From source:** `go install github.com/hpkotak/shellbud@latest` (binary is named `shellbud`; rename to `sb`). For AFM support, also run `make install-bridge` (requires macOS 26+ and Xcode 26)

### Required Secrets

| Secret | Scope | Purpose |
|--------|-------|---------|
| `GITHUB_TOKEN` | Automatic in Actions | Upload release assets |
| `TAP_GITHUB_TOKEN` | Fine-grained PAT: Contents read/write on `hpkotak/homebrew-tap` | Push Homebrew formula |

### Goreleaser Version

goreleaser is pinned to `v2.13.3` in the release workflow. The `brews` config is deprecated but functional at this version. Do not bump goreleaser without verifying `brews` support.
