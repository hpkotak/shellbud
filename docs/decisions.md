# Architecture Decision Records

## ADR-001: Regex over AST parsing for the safety classifier

**Status:** Accepted
**Date:** 2026-02-18

### Context

The safety classifier (`internal/safety/`) must detect destructive shell commands before
execution. Two approaches exist:

1. **Regex on raw command strings** — current approach
2. **Shell AST parsing** via `mvdan.cc/sh` — parses the command into a syntax tree, then
   walks `Redirect` nodes to inspect targets

`mvdan.cc/sh` was evaluated when fixing a known bypass in the `/dev/` redirection pattern.

### Decision

Stay with the regex approach. Do not add `mvdan.cc/sh`.

### Rationale

AST parsing correctly handles shell syntax (quoted strings, redirect operators, etc.) but
introduces false negatives for dynamic execution patterns that regex catches:

| Case | Regex | AST |
|---|---|---|
| `echo 'rm this'` | False positive (acceptable) | Correct (safe) |
| `echo data > /dev/sda` | Fixed by corrected pattern | Correct (destructive) |
| `bash -c "rm -rf /"` | Correct (destructive) | **False negative (safe)** |
| `eval "rm -rf /"` | Correct (destructive) | **False negative (safe)** |

For a fail-closed safety classifier, false negatives from `bash -c` and `eval` are a worse
failure mode than false positives on quoted strings. A hybrid (AST + regex) would add the
dependency without eliminating regex. Minimal dependencies is an explicit project constraint
(see CLAUDE.md).

The root bug (`\b>` not matching space-delimited redirections) was a fixable pattern error,
not an architectural limitation of the regex approach.

### Consequences

- No new dependencies introduced
- Classifier remains deterministic, fast, and model-independent
- `bash -c`, `eval`, and variable-expansion cases are caught by matching against the full
  command string (regex sees inside string arguments; AST does not)
- False positives on quoted strings containing destructive keywords remain acceptable —
  fail-closed is the stated design goal
