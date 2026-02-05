# bz

A CLI for Bzlmod - Bazel's module system.

Manage MODULE.bazel dependencies, query the Bazel Central Registry, and streamline your Bazel module workflow.

## Features

- **Module Management** - Add, remove, list, and update dependencies in MODULE.bazel
- **Dependency Insights** - Visualize dependency graphs, analyze why modules are included
- **Security Scanning** - Audit dependencies for vulnerabilities, check license compliance
- **SBOM Generation** - Generate SPDX or CycloneDX software bills of materials
- **Air-gap Support** - Full offline mode with local cache for disconnected environments
- **Registry Sync** - Mirror modules from BCR to internal registries with Starlark config

## Installation

```bash
go install github.com/albertocavalcante/bz@latest
```

Or download from [Releases](https://github.com/albertocavalcante/bz/releases).

## Quick Start

```bash
# Initialize a new module
bz init --name=my_project

# Add dependencies
bz mod add rules_go@0.50.1 rules_python@0.35.0

# List dependencies
bz mod list

# Check for updates
bz mod outdated

# Update all dependencies
bz mod update

# View dependency graph
bz mod graph

# Scan for vulnerabilities
bz audit
```

## Commands

### Module Initialization

| Command   | Description                        |
| --------- | ---------------------------------- |
| `bz init` | Initialize a new MODULE.bazel file |

```bash
bz init                          # use directory name as module name
bz init --name=my_module         # set module name
bz init --version=1.0.0          # set initial version
bz init --force                  # overwrite existing MODULE.bazel
```

### Dependency Management

| Command           | Description                            |
| ----------------- | -------------------------------------- |
| `bz mod add`      | Add dependencies to MODULE.bazel       |
| `bz mod rm`       | Remove dependencies from MODULE.bazel  |
| `bz mod list`     | List all dependencies                  |
| `bz mod info`     | Show module information from registry  |
| `bz mod update`   | Update dependencies to latest versions |
| `bz mod outdated` | Check for newer versions               |
| `bz mod search`   | Search for modules in registry         |
| `bz mod sync`     | Sync modules between registries        |

```bash
# Add dependencies
bz mod add rules_go@0.50.1
bz mod add --dev gazelle@0.38.0

# Remove dependencies
bz mod rm rules_go
bz mod rm --dry-run rules_python

# List dependencies
bz mod list
bz mod list --all --json

# Update dependencies
bz mod update                    # update all
bz mod update rules_go           # update specific module
bz mod update --dry-run          # preview changes

# Search registry
bz mod search rules_go
bz mod search python -n 10       # limit results
```

### Dependency Analysis

| Command        | Description                      |
| -------------- | -------------------------------- |
| `bz mod graph` | Display dependency graph         |
| `bz mod stats` | Show dependency statistics       |
| `bz mod why`   | Explain why a module is included |

```bash
# Dependency graph
bz mod graph                     # ASCII tree
bz mod graph --format=dot        # Graphviz DOT
bz mod graph --format=mermaid    # Mermaid diagram
bz mod graph --depth=2           # limit depth
bz mod graph --json              # JSON output

# Statistics
bz mod stats                     # show counts and depth
bz mod stats --json

# Dependency path
bz mod why protobuf              # why is protobuf included?
bz mod why protobuf --all        # show all paths
```

### Security & Compliance

| Command           | Description                        |
| ----------------- | ---------------------------------- |
| `bz audit`        | Scan for vulnerabilities (via OSV) |
| `bz mod licenses` | Show license information           |
| `bz sbom`         | Generate SBOM (SPDX/CycloneDX)     |

```bash
# Vulnerability scanning
bz audit                         # scan all dependencies
bz audit --severity=high         # only high/critical
bz audit --fix                   # show fix suggestions
bz audit --ecosystem=Go          # specify ecosystem

# License compliance
bz mod licenses                  # list all licenses
bz mod licenses --summary        # show license counts
bz mod licenses --check --deny=GPL-3.0
bz mod licenses --check --allow=MIT,Apache-2.0

# SBOM generation
bz sbom                          # SPDX format to stdout
bz sbom --format=cyclonedx       # CycloneDX format
bz sbom --output=sbom.json       # write to file
```

### Cache Management (Air-gap Support)

| Command             | Description                     |
| ------------------- | ------------------------------- |
| `bz cache download` | Download modules to local cache |
| `bz cache stats`    | Show cache statistics           |
| `bz cache verify`   | Verify cache completeness       |
| `bz cache clear`    | Clear local cache               |

```bash
# Download for offline use
bz cache download                # cache deps from MODULE.bazel
bz cache download rules_go       # cache specific module
bz cache download --all          # cache ALL modules (large!)

# Cache info
bz cache stats                   # show cache size
bz cache verify                  # check if cache is complete

# Clear cache
bz cache clear                   # clear all (with confirmation)
bz cache clear --force           # skip confirmation
bz cache clear rules_go          # clear specific module
```

### Utilities

| Command            | Description                |
| ------------------ | -------------------------- |
| `bz doctor`        | Check Bazel/bzlmod setup   |
| `bz registry ping` | Test registry connectivity |
| `bz completion`    | Generate shell completions |
| `bz version`       | Show version information   |

```bash
# Health check
bz doctor                        # check Bazel setup
bz doctor --json

# Registry connectivity
bz registry ping                 # test default BCR
bz registry ping https://my.registry

# Shell completion
bz completion bash > /etc/bash_completion.d/bz
bz completion zsh > "${fpath[1]}/_bz"
bz completion fish > ~/.config/fish/completions/bz.fish
```

## Global Flags

These flags work with all commands:

| Flag               | Description                                        |
| ------------------ | -------------------------------------------------- |
| `-q`, `--quiet`    | Reduce output, only show errors and essential info |
| `--no-color`       | Disable colored output                             |
| `--offline`        | Disable network access, use cache only             |
| `--prefer-offline` | Prefer cache, fallback to network if needed        |
| `--registry`       | Override registry URL                              |
| `-h`, `--help`     | Show help for any command                          |

```bash
bz mod list --quiet              # minimal output
bz mod info rules_go --no-color  # no ANSI colors
bz mod list --offline            # cache only
bz mod outdated --prefer-offline # cache first
bz mod search --registry=https://my.registry rules_go
```

## Configuration

bz supports configuration via TOML files and environment variables.

### Configuration Files

| Location                   | Purpose               |
| -------------------------- | --------------------- |
| `~/.config/bz/config.toml` | User configuration    |
| `.bz.toml`                 | Project configuration |

Example `config.toml`:

```toml
[network]
mode = "prefer-offline" # "online", "prefer-offline", or "offline"
registry = "https://bcr.bazel.build"
timeout = "30s"

[cache]
dir = "~/.cache/bz"
ttl = "24h"

[commands]
disabled = ["audit"] # disable specific commands
```

### Environment Variables

| Variable              | Description                              |
| --------------------- | ---------------------------------------- |
| `BZ_OFFLINE`          | Enable offline mode (`1`, `true`, `yes`) |
| `BZ_PREFER_OFFLINE`   | Enable prefer-offline mode               |
| `BZ_REGISTRY`         | Override registry URL                    |
| `BZ_CACHE_DIR`        | Override cache directory                 |
| `BZ_DISABLE_COMMANDS` | Disable commands (comma-separated)       |

### Precedence

Configuration is loaded in this order (later overrides earlier):

1. Built-in defaults
2. System config (`/etc/bz/config.toml`)
3. User config (`~/.config/bz/config.toml`)
4. Project config (`.bz.toml`)
5. Environment variables
6. CLI flags

## Air-gap / Offline Usage

bz fully supports air-gapped environments. See the [Air-gap Guide](docs/src/content/docs/guides/air-gapped.mdx) for details.

**Quick setup:**

```bash
# On connected machine: download dependencies
bz cache download

# Transfer cache to air-gapped machine
rsync -av ~/.cache/bz/ user@airgap:~/.cache/bz/

# On air-gapped machine: verify and use
bz cache verify
export BZ_OFFLINE=1
bz mod list
```

## Registry Sync (Starlark Config)

For advanced registry mirroring, bz supports Starlark-based configuration:

```starlark
# bz.star
bcr = registry.http("https://bcr.bazel.build")
mirror = registry.file("/path/to/mirror")

sync.workflow(
    name = "mirror-essentials",
    origin = bcr,
    destination = mirror,
    modules = ["rules_go", "rules_python"],
    versions = sync.latest(count = 3),
)
```

```bash
bz mod sync mirror-essentials
bz mod sync --list               # list available workflows
bz mod sync --dry-run            # preview sync
```

## Documentation

- [Full Documentation](https://albertocavalcante.github.io/bz/)
- [CLI Reference](docs/src/content/docs/cli/)
- [Configuration Guide](docs/src/content/docs/configuration/)
- [Air-gap Guide](docs/src/content/docs/guides/air-gapped.mdx)

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or [MIT license](LICENSE-MIT) at your option.
