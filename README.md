# bz

[![CI](https://github.com/albertocavalcante/bz/actions/workflows/ci.yml/badge.svg)](https://github.com/albertocavalcante/bz/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertocavalcante/bz)](https://goreportcard.com/report/github.com/albertocavalcante/bz)
[![Go Reference](https://pkg.go.dev/badge/github.com/albertocavalcante/bz.svg)](https://pkg.go.dev/github.com/albertocavalcante/bz)
[![License](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue)](LICENSE-MIT)
[![Release](https://img.shields.io/github/v/release/albertocavalcante/bz)](https://github.com/albertocavalcante/bz/releases/latest)

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

Or download a binary from [Releases](https://github.com/albertocavalcante/bz/releases).

## Quick Start

```bash
# Initialize a new module
bz init --name=my_project

# Add dependencies
bz mod add rules_go@0.50.1 rules_python@0.35.0

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

| Command             | Description                            |
| ------------------- | -------------------------------------- |
| `bz init`           | Initialize a new MODULE.bazel file     |
| `bz mod add`        | Add dependencies                       |
| `bz mod rm`         | Remove dependencies                    |
| `bz mod list`       | List all dependencies                  |
| `bz mod info`       | Show module information from registry  |
| `bz mod update`     | Update dependencies to latest versions |
| `bz mod outdated`   | Check for newer versions               |
| `bz mod search`     | Search for modules in registry         |
| `bz mod sync`       | Sync modules between registries        |
| `bz mod graph`      | Display dependency graph               |
| `bz mod stats`      | Show dependency statistics             |
| `bz mod why`        | Explain why a module is included       |
| `bz mod licenses`   | Show license information               |
| `bz audit`          | Scan for vulnerabilities (via OSV)     |
| `bz sbom`           | Generate SBOM (SPDX/CycloneDX)         |
| `bz cache download` | Download modules to local cache        |
| `bz cache verify`   | Verify cache completeness              |
| `bz cache clear`    | Clear local cache                      |
| `bz doctor`         | Check Bazel/bzlmod setup               |
| `bz registry ping`  | Test registry connectivity             |
| `bz completion`     | Generate shell completions             |

Run `bz <command> --help` for usage details, or see the [CLI Reference](https://albertocavalcante.github.io/bz/cli/).

## Configuration

bz reads configuration from TOML files and environment variables:

| Location                   | Purpose               |
| -------------------------- | --------------------- |
| `~/.config/bz/config.toml` | User configuration    |
| `.bz.toml`                 | Project configuration |

```toml
[network]
mode = "prefer-offline"
registry = "https://bcr.bazel.build"
timeout = "30s"

[cache]
dir = "~/.cache/bz"
ttl = "24h"
```

See the [Configuration Guide](https://albertocavalcante.github.io/bz/configuration/) for environment variables, precedence rules, and Starlark-based config.

## Documentation

- [Full Documentation](https://albertocavalcante.github.io/bz/)
- [Getting Started](https://albertocavalcante.github.io/bz/getting-started/)
- [CLI Reference](https://albertocavalcante.github.io/bz/cli/)
- [Configuration](https://albertocavalcante.github.io/bz/configuration/)
- [Air-gapped Setup](https://albertocavalcante.github.io/bz/guides/air-gapped/)
- [Registry Mirroring](https://albertocavalcante.github.io/bz/guides/mirror-bcr/)

## Contributing

See [CONTRIBUTING.md](.github/CONTRIBUTING.md) to get started.

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or [MIT license](LICENSE-MIT) at your option.
