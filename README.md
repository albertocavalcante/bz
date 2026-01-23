# bz

CLI for Bzlmod - Bazel's module system.

## Installation

```bash
go install github.com/albertocavalcante/bz@latest
```

## Usage

```bash
# Add a dependency
bz mod add rules_go@0.50.1
bz mod add rules_go@0.50.1 rules_python@0.35.0
bz mod add --dev gazelle@0.38.0

# List dependencies
bz mod list
bz mod list --json

# Get module info from registry
bz mod info rules_go@0.50.1
bz mod info rules_go@0.50.1 --json

# Search the Bazel Central Registry
bz mod search rules_python

# Use a custom registry
bz mod --registry https://my-bcr.example.com list
```

## Commands

| Command      | Description                           |
| ------------ | ------------------------------------- |
| `mod add`    | Add dependencies to MODULE.bazel      |
| `mod list`   | List dependencies in MODULE.bazel     |
| `mod info`   | Show module information from registry |
| `mod search` | Search for modules in the BCR         |
| `version`    | Print version information             |

### Global Flags

| Flag         | Description                                                  |
| ------------ | ------------------------------------------------------------ |
| `--registry` | Custom Bazel Central Registry URL (default: bcr.bazel.build) |

## License

Licensed under either of [Apache License, Version 2.0](LICENSE-APACHE) or [MIT license](LICENSE-MIT) at
your option.
