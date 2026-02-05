# bz Command Reference

Complete reference for all bz commands.

## Table of Contents

- [Global Flags](#global-flags)
- [Module Initialization](#module-initialization)
- [Module Management](#module-management)
- [Dependency Analysis](#dependency-analysis)
- [Security & Compliance](#security--compliance)
- [Cache Management](#cache-management)
- [Utilities](#utilities)

---

## Global Flags

These flags are available for all commands:

| Flag               | Description                                           |
| ------------------ | ----------------------------------------------------- |
| `-q`, `--quiet`    | Reduce output, only show errors and essential info    |
| `--no-color`       | Disable colored output (auto-detected when not a TTY) |
| `--offline`        | Disable all network access, use cache only            |
| `--prefer-offline` | Prefer cached data, fallback to network if needed     |
| `--registry <url>` | Override the default registry URL                     |
| `-h`, `--help`     | Show help for any command                             |
| `-v`, `--version`  | Show version information (on root command)            |

```bash
bz mod list --quiet              # minimal output
bz mod info rules_go --no-color  # no ANSI colors
bz mod list --offline            # cache only
bz mod outdated --prefer-offline # cache first
bz mod search --registry=https://my.registry rules_go
```

---

## Module Initialization

### `bz init`

Initialize a new Bazel module by creating a MODULE.bazel file.

```bash
bz init                          # use directory name as module name
bz init --name=my_module         # set module name
bz init --name=foo --version=1.0.0
bz init --force                  # overwrite existing MODULE.bazel
```

| Flag        | Description                               |
| ----------- | ----------------------------------------- |
| `--name`    | Module name (defaults to directory name)  |
| `--version` | Initial module version (default: `0.0.0`) |
| `--force`   | Overwrite existing MODULE.bazel           |

---

## Module Management

### `bz mod add`

Add one or more dependencies to MODULE.bazel.

```bash
bz mod add rules_go@0.50.1
bz mod add rules_go@0.50.1 rules_python@0.35.0
bz mod add --dev gazelle@0.38.0
```

| Flag    | Description             |
| ------- | ----------------------- |
| `--dev` | Add as a dev dependency |

### `bz mod rm`

Remove dependencies from MODULE.bazel.

```bash
bz mod rm rules_go
bz mod rm rules_go rules_python
bz mod rm --dry-run rules_go
```

| Flag        | Description                                       |
| ----------- | ------------------------------------------------- |
| `--dry-run` | Show what would be removed without making changes |

### `bz mod list`

List dependencies in MODULE.bazel.

```bash
bz mod list
bz mod list --all     # include extensions, overrides, toolchains
bz mod list --json
```

| Flag          | Description                                     |
| ------------- | ----------------------------------------------- |
| `-a`, `--all` | Show all contents (extensions, overrides, etc.) |
| `--json`      | Output as JSON                                  |

### `bz mod info`

Show information about a module from the registry.

```bash
bz mod info rules_go            # show metadata and all versions
bz mod info rules_go@0.50.1     # show specific version details
bz mod info rules_go --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

### `bz mod update`

Update dependencies to latest stable versions.

```bash
bz mod update                    # update all dependencies
bz mod update rules_go           # update specific module
bz mod update rules_go gazelle   # update multiple modules
bz mod update --dry-run          # preview changes
```

| Flag        | Description                                       |
| ----------- | ------------------------------------------------- |
| `--dry-run` | Show what would be updated without making changes |

### `bz mod outdated`

Check all dependencies for newer versions.

```bash
bz mod outdated
bz mod outdated --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

### `bz mod search`

Search for modules in the registry.

```bash
bz mod search rules_go
bz mod search python
bz mod search protobuf -v        # show version info
bz mod search grpc -n 5          # limit to 5 results
```

| Flag              | Description                             |
| ----------------- | --------------------------------------- |
| `-v`, `--verbose` | Show version info for each result       |
| `-n`, `--limit`   | Maximum number of results (default: 20) |

### `bz mod sync`

Sync modules from a source registry to a destination based on a workflow defined in `bz.star` configuration.

```bash
bz mod sync mirror-essential           # run a workflow
bz mod sync mirror-essential --dry-run # preview what would be synced
bz mod sync --list                     # list available workflows
```

| Flag              | Description                                      |
| ----------------- | ------------------------------------------------ |
| `-c`, `--config`  | Path to config file (default: `bz.star`)         |
| `--dry-run`       | Show what would be synced without making changes |
| `-l`, `--list`    | List available workflows                         |
| `--modules`       | Override modules to sync (comma-separated)       |
| `-v`, `--verbose` | Print detailed progress                          |

---

## Dependency Analysis

### `bz mod graph`

Display the dependency graph of your Bazel module.

```bash
bz mod graph                     # ASCII tree output
bz mod graph --format=dot        # Graphviz DOT format
bz mod graph --format=mermaid    # Mermaid diagram
bz mod graph --format=json       # JSON structure
bz mod graph --json              # shortcut for --format=json
bz mod graph --depth=2           # limit depth
```

| Flag       | Description                                                |
| ---------- | ---------------------------------------------------------- |
| `--format` | Output format: `ascii` (default), `dot`, `json`, `mermaid` |
| `--json`   | Shortcut for `--format=json`                               |
| `--depth`  | Maximum depth to traverse (0 = unlimited)                  |

**Generate visualization:**

```bash
bz mod graph --format=dot | dot -Tpng -o deps.png
```

### `bz mod stats`

Display statistics about module dependencies.

```bash
bz mod stats
bz mod stats --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

### `bz mod why`

Show the dependency path(s) that bring a module into your project.

```bash
bz mod why protobuf              # why is protobuf included?
bz mod why protobuf --all        # show all paths, not just shortest
bz mod why protobuf --json
```

| Flag     | Description                                   |
| -------- | --------------------------------------------- |
| `--all`  | Show all paths (default: shortest paths only) |
| `--json` | Output as JSON                                |

---

## Security & Compliance

### `bz audit`

Scan dependencies for security vulnerabilities using the OSV database.

```bash
bz audit                         # scan all dependencies
bz audit --json                  # output as JSON
bz audit --severity=high         # only high/critical vulnerabilities
bz audit --fix                   # show suggested updates
bz audit --ecosystem=Go          # specify ecosystem manually
```

| Flag          | Description                                                              |
| ------------- | ------------------------------------------------------------------------ |
| `--json`      | Output as JSON                                                           |
| `--severity`  | Minimum severity to report: `critical`, `high`, `medium`, `low`          |
| `--fix`       | Show suggested updates to fix vulnerabilities                            |
| `--ecosystem` | OSV ecosystem to query (e.g., `Go`, `PyPI`, `crates.io`, `npm`, `Maven`) |

**Note:** OSV does not have a "Bazel" ecosystem. bz maps known Bazel modules to their underlying ecosystems:

- `rules_go`, `gazelle` -> Go
- `rules_python` -> PyPI
- `rules_rust` -> crates.io
- `rules_nodejs` -> npm
- `rules_java` -> Maven

### `bz mod licenses`

Display license information for all dependencies.

```bash
bz mod licenses                          # list all licenses
bz mod licenses --json                   # output as JSON
bz mod licenses --summary                # show license counts only
bz mod licenses --check --deny=GPL-3.0   # check for denied licenses
bz mod licenses --check --allow=MIT,Apache-2.0
```

| Flag        | Description                                                    |
| ----------- | -------------------------------------------------------------- |
| `--json`    | Output as JSON                                                 |
| `--summary` | Show license summary only                                      |
| `--check`   | Check licenses against policy (requires `--allow` or `--deny`) |
| `--allow`   | Comma-separated list of allowed licenses                       |
| `--deny`    | Comma-separated list of denied licenses                        |

### `bz sbom`

Generate a Software Bill of Materials (SBOM) for your Bazel module.

```bash
bz sbom                          # SPDX 2.3 JSON to stdout
bz sbom --format=cyclonedx       # CycloneDX 1.4 JSON
bz sbom --output=sbom.json       # write to file
bz sbom --include-transitive=false
```

| Flag                   | Description                                     |
| ---------------------- | ----------------------------------------------- |
| `--format`             | Output format: `spdx` (default), `cyclonedx`    |
| `--output`             | Output file path (default: stdout)              |
| `--include-transitive` | Include transitive dependencies (default: true) |
| `--registry`           | Registry URL                                    |

---

## Cache Management

### `bz cache download`

Download modules and their transitive dependencies to the local cache.

```bash
bz cache download                    # cache deps from MODULE.bazel
bz cache download rules_go gazelle   # cache specific modules
bz cache download rules_go@0.50.1    # cache specific version
bz cache download --all              # cache ALL modules (large!)
```

| Flag          | Description                                              |
| ------------- | -------------------------------------------------------- |
| `--all`       | Download ALL modules from registry (warning: very large) |
| `--json`      | Output as JSON                                           |
| `--registry`  | Registry URL to download from                            |
| `--cache-dir` | Cache directory (default: `~/.cache/bz`)                 |

### `bz cache stats`

Show statistics about the local module cache.

```bash
bz cache stats
bz cache stats --json
bz cache stats --cache-dir=/path/to/cache
```

| Flag          | Description                              |
| ------------- | ---------------------------------------- |
| `--json`      | Output as JSON                           |
| `--cache-dir` | Cache directory (default: `~/.cache/bz`) |

### `bz cache verify`

Verify that the local cache contains all dependencies required for offline use.

```bash
bz cache verify
bz cache verify --json
bz cache verify --cache-dir=/path/to/cache
```

| Flag          | Description                              |
| ------------- | ---------------------------------------- |
| `--json`      | Output as JSON                           |
| `--cache-dir` | Cache directory (default: `~/.cache/bz`) |

### `bz cache clear`

Clear the local module cache.

```bash
bz cache clear                   # clear all (with confirmation)
bz cache clear --force           # skip confirmation
bz cache clear rules_go          # clear specific module
bz cache clear --json
```

| Flag            | Description                              |
| --------------- | ---------------------------------------- |
| `-f`, `--force` | Skip confirmation prompt                 |
| `--json`        | Output as JSON                           |
| `--cache-dir`   | Cache directory (default: `~/.cache/bz`) |

---

## Utilities

### `bz doctor`

Check Bazel/bzlmod setup and diagnose common issues.

```bash
bz doctor
bz doctor --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

**Checks performed:**

- Bazel installation and version
- MODULE.bazel file existence
- .bazelversion file existence
- Bzlmod enabled status (Bazel 7+ has it on by default)
- Registry connectivity

### `bz registry ping`

Test registry connectivity.

```bash
bz registry ping                       # check default BCR
bz registry ping https://my.registry   # check specific registry
bz registry ping --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

### `bz completion`

Generate shell completion scripts.

**Bash:**

```bash
# Linux
bz completion bash > /etc/bash_completion.d/bz

# macOS (Homebrew)
bz completion bash > $(brew --prefix)/etc/bash_completion.d/bz
```

**Zsh:**

```bash
echo "autoload -U compinit; compinit" >> ~/.zshrc
bz completion zsh > "${fpath[1]}/_bz"
```

**Fish:**

```bash
bz completion fish > ~/.config/fish/completions/bz.fish
```

**PowerShell:**

```powershell
bz completion powershell | Out-String | Invoke-Expression
bz completion powershell >> $PROFILE
```

### `bz version`

Print version information.

```bash
bz version
bz version --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

---

## Exit Codes

| Code | Description                                             |
| ---- | ------------------------------------------------------- |
| 0    | Success                                                 |
| 1    | General error (command failed, invalid arguments, etc.) |

Commands that perform checks (`audit`, `cache verify`, `mod licenses --check`) return non-zero exit codes when issues are found, making them suitable for CI/CD pipelines.
