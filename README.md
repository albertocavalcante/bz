# bz

CLI for Bzlmod - Bazel's module system.

Manage MODULE.bazel dependencies, query the Bazel Central Registry, and streamline your Bazel module workflow.

## Installation

```bash
go install github.com/albertocavalcante/bz@latest
```

## Quick Start

```bash
# Initialize a new module
bz init --name=my_project

# Add dependencies
bz mod add rules_go@0.50.1

# List dependencies
bz mod list

# Check for updates
bz mod outdated

# Update all dependencies
bz mod update
```

## Commands

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

### `bz mod add`

Add one or more dependencies to MODULE.bazel.

```bash
bz mod add rules_go@0.50.1
bz mod add rules_go@0.50.1 rules_python@0.35.0
bz mod add --dev gazelle@0.38.0
```

| Flag    | Description           |
| ------- | --------------------- |
| `--dev` | Add as dev dependency |

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

### `bz registry ping`

Check registry availability.

```bash
bz registry ping                       # check default BCR
bz registry ping https://my.registry   # check specific registry
bz registry ping --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

### `bz version`

Print version information.

```bash
bz version
bz version --json
```

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

## Global Flags

These flags work with all commands:

| Flag            | Description                                        |
| --------------- | -------------------------------------------------- |
| `-q`, `--quiet` | Reduce output, only show errors and essential info |
| `--no-color`    | Disable colored output                             |
| `-h`, `--help`  | Help for any command                               |

## Registry Flag

The `mod` subcommands support a custom registry:

| Flag         | Description                                                                                                  |
| ------------ | ------------------------------------------------------------------------------------------------------------ |
| `--registry` | Registry URL - supports `https://`, `http://`, `file://`, or local path (default: `https://bcr.bazel.build`) |

```bash
bz mod list --registry=https://my-bcr.example.com
bz mod search rules --registry=/path/to/local/registry
```

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or [MIT license](LICENSE-MIT) at your option.
