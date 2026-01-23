# bz

CLI for Bzlmod - Bazel's module system.

## Installation

```bash
go install github.com/albertocavalcante/bz@latest
```

## Usage

```bash
# Add a dependency
bz add rules_go
bz add rules_go@0.50.1

# List dependencies
bz list
bz list --outdated

# Search the Bazel Central Registry
bz search rules_python

# Get module info
bz info rules_go
```

## Commands

| Command   | Description                       |
| --------- | --------------------------------- |
| `add`     | Add a dependency to MODULE.bazel  |
| `list`    | List dependencies in MODULE.bazel |
| `search`  | Search for modules in the BCR     |
| `info`    | Show information about a module   |
| `version` | Print version information         |

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or [MIT license](LICENSE-MIT) at
your option.
