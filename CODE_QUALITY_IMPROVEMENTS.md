# Code Quality Improvement Opportunities

Date: 2026-02-17

## Review method

- Ran full validation: `go build ./...`, `go test -count=1 ./...`, `golangci-lint`.
- Collected coverage and low-coverage function hotspots from `go tool cover -func`.
- Reviewed structural hotspots (file size, repetition patterns, test isolation patterns).

## Top opportunities (prioritized)

### 1) Normalize command-disable keys and align docs with runtime checks (High)

- Evidence:
  - Runtime checks use short keys like `"ping"` (`cmd/registry/ping.go:67`) and `"download"` (`cmd/cache/download.go:67`).
  - Docs show disabling full command paths like `disabled = ["registry ping"]` (`docs/src/content/docs/configuration/toml.mdx:169`, `docs/src/content/docs/guides/air-gapped.mdx:306`, `docs/markdown/configuration.md:171`, `docs/markdown/air-gap.md:312`).
  - Disable logic is exact string match (`internal/bzconfig/config.go:177`).
- Risk:
  - User config may silently fail to disable intended commands.
  - Future subcommand name collisions (`download`, `search`, etc.) are likely.
- Recommendation:
  - Adopt canonical command-path keys (`registry ping`, `cache download`) using `cmd.CommandPath()` mapping.
  - Support backward-compatible aliases for current short names.
  - Add validation warnings for unknown/unused disable entries.

### 2) Improve cmd test isolation and parallelism (High)

- Evidence:
  - `os.Chdir` usage in `cmd/*_test.go`: **224 occurrences**.
  - `t.Parallel()` usage in `cmd/*_test.go`: **1 occurrence**.
  - `t.Parallel()` usage in `internal/*_test.go`: **807 occurrences**.
  - Biggest offenders include `cmd/mod/update_test.go` (34), `cmd/mod/add_test.go` (22), `cmd/mod/licenses_test.go` (22), `cmd/mod/graph_test.go` (20), `cmd/init_test.go` (20).
- Risk:
  - Slower CI and less deterministic test behavior.
  - Hidden shared-state coupling in command tests.
- Recommendation:
  - Introduce test helpers to avoid process-wide cwd mutation (use temp module roots + explicit path args where possible).
  - Partition tests into parallel-safe groups and enable `t.Parallel()` incrementally.

### 3) Add tests for newly introduced shared helpers (High)

- Evidence:
  - `internal/cmdutil` package coverage is 0.0% (`internal/cmdutil/context.go`, `internal/cmdutil/defaults.go`, `internal/cmdutil/subcommand.go`).
  - `internal/starlark/convertutil` package coverage is 0.0% (`internal/starlark/convertutil/go_value.go`).
- Risk:
  - Shared behavior now fan-outs into many commands/packages without direct unit coverage.
- Recommendation:
  - Add focused unit tests for:
    - `CommandContext`, `ApplyErrorSilence`, `RequireSubcommand`.
    - `convertutil.ToGoValue` option matrix (overflow mode, unknown value mode, nested list/dict/tuple).

### 4) Refactor large command files into domain/presentation layers (Medium)

- Evidence:
  - `cmd/sbom.go` is 470 LOC and mixes CLI flag handling, graph traversal, SPDX/CycloneDX models, and serialization.
  - `cmd/doctor.go` is 464 LOC and blends checks, formatting, and output mapping.
- Risk:
  - Lower maintainability and harder targeted tests.
  - More merge conflicts when evolving features.
- Recommendation:
  - Move format model/types and renderers to `internal/sbom/*` and `internal/doctor/*`.
  - Keep `cmd/*` files as thin wiring layers.

### 5) Consolidate repeated pretty-JSON encoding path (Medium)

- Evidence:
  - `json.NewEncoder(...)` appears **23 times** across `cmd/` and `internal/`.
  - Indented JSON setup `SetIndent("", "  ")` appears **18 times**.
- Risk:
  - Inconsistent behavior creep (indentation, newline handling, error wrapping).
- Recommendation:
  - Add shared helper(s), e.g. `internal/cli/jsonutil.WritePretty(w, v)` and `WriteCompact`.
  - Migrate command outputs incrementally.

### 6) Raise coverage in key low-coverage runtime packages (Medium)

- Evidence:
  - Package coverage: `internal/cli` 49.2%, `internal/modsync` 71.4%, `internal/tui` 74.7%, `internal/starlark/value` 75.8%.
  - 0% function hotspots include spinner paths (`internal/cli/spinner.go`), cmdutil helpers, and conversion utility functions.
  - `internal/modsync/runner.go:317` (`publisherFromConfig`) is ~23.1%.
- Risk:
  - Behavior regressions in operational flows that are hard to detect pre-release.
- Recommendation:
  - Add table-driven tests for spinner lifecycle and effect-of-config helpers.
  - Expand `modsync` coverage for `publisherFromConfig` branches and failure modes.

### 7) Complete or gate intentionally partial implementation paths (Medium)

- Evidence:
  - `internal/modsync/runner.go:277` (`filterSince`) currently returns all versions (stub behavior; 0% coverage).
- Risk:
  - Feature appears supported but may violate user expectations.
- Recommendation:
  - Either implement date-based filtering (with metadata constraints documented), or mark as explicitly unsupported with clear CLI/runtime messaging.

### 8) Long-term config model convergence (Low/Strategic)

- Evidence:
  - Two config systems remain active: TOML (`internal/bzconfig`) and Starlark (`internal/config` + modules).
- Risk:
  - Concept duplication and precedence ambiguity over time.
- Recommendation:
  - Publish a formal precedence matrix and migration direction.
  - Consider introducing a single merged runtime config view with source attribution.

## Suggested execution order

1. Command-disable normalization + docs alignment.
2. Add unit tests for `cmdutil` and `convertutil`.
3. Cmd test isolation/parallelism migration for top offenders.
4. `modsync` coverage and `filterSince` decision/implementation.
5. SBOM/Doctor structural extraction.
6. JSON writer helper standardization.

## Definition of done (for next quality cycle)

- No config/doc mismatches for command disable keys.
- `internal/cmdutil` and `internal/starlark/convertutil` both >= 90% coverage.
- `cmd` test suite reduces `os.Chdir` usage by at least 50% and materially increases `t.Parallel()` usage.
- `internal/cli` coverage > 70% and `internal/modsync` coverage > 80%.
