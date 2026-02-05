# bz Configuration Guide

bz supports configuration via TOML files and environment variables for persistent settings.

## Table of Contents

- [Configuration Files](#configuration-files)
- [Configuration Sections](#configuration-sections)
- [Environment Variables](#environment-variables)
- [Configuration Precedence](#configuration-precedence)
- [Common Configurations](#common-configurations)

---

## Configuration Files

bz looks for configuration in these locations (in order of precedence):

| Location                   | Purpose                   |
| -------------------------- | ------------------------- |
| `/etc/bz/config.toml`      | System-wide defaults      |
| `~/.config/bz/config.toml` | User preferences          |
| `.bz.toml`                 | Project-specific settings |

Settings in later files override earlier ones. CLI flags and environment variables override all file-based configuration.

## Full Example

```toml
# ~/.config/bz/config.toml

# Network settings
[network]
mode = "prefer-offline" # "online", "prefer-offline", or "offline"
registry = "https://bcr.bazel.build"
fallback_registries = [
  "https://mirror.internal.com/bcr",
]
timeout = "30s"

# Cache settings
[cache]
dir = "~/.cache/bz" # Cache directory
ttl = "24h" # Cache time-to-live (optional)

# Command settings
[commands]
disabled = ["audit"] # Disable specific commands
```

---

## Configuration Sections

### `[network]`

Controls how bz accesses registries.

| Key                   | Type   | Default                     | Description                                            |
| --------------------- | ------ | --------------------------- | ------------------------------------------------------ |
| `mode`                | string | `"online"`                  | Network mode: `online`, `prefer-offline`, or `offline` |
| `registry`            | string | `"https://bcr.bazel.build"` | Primary registry URL                                   |
| `fallback_registries` | array  | `[]`                        | Backup registries to try if primary fails              |
| `timeout`             | string | `"30s"`                     | Request timeout (Go duration format)                   |

**Network modes:**

| Mode             | Behavior                                    |
| ---------------- | ------------------------------------------- |
| `online`         | Always fetch from network (default)         |
| `prefer-offline` | Use cache if available, fallback to network |
| `offline`        | Never access network, fail if not cached    |

```toml
[network]
mode = "offline"
registry = "file:///opt/bazel-registry"
```

### `[cache]`

Controls the local module cache.

| Key   | Type   | Default       | Description                                     |
| ----- | ------ | ------------- | ----------------------------------------------- |
| `dir` | string | `~/.cache/bz` | Cache directory path                            |
| `ttl` | string | `""`          | Cache entry time-to-live (empty = never expire) |

```toml
[cache]
dir = "/var/cache/bz"
ttl = "7d"
```

### `[commands]`

Controls command availability.

| Key        | Type  | Default | Description         |
| ---------- | ----- | ------- | ------------------- |
| `disabled` | array | `[]`    | Commands to disable |

```toml
[commands]
disabled = ["audit", "sbom"]
```

Disabled commands will return an error when invoked. This is useful for restricted environments where certain commands should not be available.

---

## Environment Variables

Environment variables provide another way to configure bz, especially useful in CI/CD pipelines and containers.

| Variable              | Equivalent Config                 | Description                        |
| --------------------- | --------------------------------- | ---------------------------------- |
| `BZ_OFFLINE`          | `network.mode = "offline"`        | Enable offline mode                |
| `BZ_PREFER_OFFLINE`   | `network.mode = "prefer-offline"` | Enable prefer-offline mode         |
| `BZ_REGISTRY`         | `network.registry`                | Override registry URL              |
| `BZ_CACHE_DIR`        | `cache.dir`                       | Override cache directory           |
| `BZ_DISABLE_COMMANDS` | `commands.disabled`               | Disable commands (comma-separated) |

**Boolean values:** `BZ_OFFLINE` and `BZ_PREFER_OFFLINE` accept `1`, `true`, or `yes` (case-insensitive).

```bash
# Enable offline mode
export BZ_OFFLINE=1

# Use custom registry
export BZ_REGISTRY="https://internal.registry.com"

# Disable specific commands
export BZ_DISABLE_COMMANDS="audit,sbom"
```

---

## Configuration Precedence

Configuration is merged in this order (later overrides earlier):

1. **Built-in defaults**
2. **System config** (`/etc/bz/config.toml`)
3. **User config** (`~/.config/bz/config.toml`)
4. **Project config** (`.bz.toml` in current directory)
5. **Environment variables**
6. **CLI flags**

This allows you to set organization-wide defaults in system config, user preferences in user config, and project-specific settings in project config.

---

## Common Configurations

### Air-gapped Environment

For machines without internet access:

```toml
# /etc/bz/config.toml (system-wide)

[network]
mode = "offline"
registry = "file:///opt/bazel/registry"

[cache]
dir = "/opt/bazel/cache"

[commands]
disabled = ["registry ping"] # No point in pinging
```

### CI/CD Pipeline

For build servers:

```toml
# .bz.toml (project config)

[network]
mode = "prefer-offline"
registry = "https://internal-mirror.company.com/bcr"
timeout = "60s"

[cache]
dir = "/tmp/bz-cache"
```

Or via environment variables in your CI config:

```yaml
# .github/workflows/build.yml
env:
  BZ_PREFER_OFFLINE: "1"
  BZ_REGISTRY: "https://internal-mirror.company.com/bcr"
  BZ_CACHE_DIR: "/tmp/bz-cache"
```

### Development Machine

For developers who want fast local builds:

```toml
# ~/.config/bz/config.toml

[network]
mode = "prefer-offline"

[cache]
dir = "~/.cache/bz"
ttl = "24h"
```

### Restricted Environment

For security-sensitive environments:

```toml
# /etc/bz/config.toml

[network]
mode = "offline"
registry = "file:///approved/registry"

[commands]
disabled = ["audit"] # Disable commands that require network
```

---

## Registry URL Formats

The `registry` setting accepts several URL formats:

| Format | Example                    | Description                           |
| ------ | -------------------------- | ------------------------------------- |
| HTTPS  | `https://bcr.bazel.build`  | Standard BCR-compatible HTTP registry |
| HTTP   | `http://internal:8080`     | Unsecured HTTP (local/internal only)  |
| File   | `file:///path/to/registry` | Local filesystem registry             |
| Path   | `/path/to/registry`        | Shorthand for file://                 |

```toml
[network]
# Standard BCR
registry = "https://bcr.bazel.build"

# Internal mirror
registry = "https://mirror.internal.company.com/bcr"

# Local filesystem
registry = "file:///opt/bazel/registry"
# or
registry = "/opt/bazel/registry"
```
