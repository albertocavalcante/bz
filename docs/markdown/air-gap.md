# Air-gapped & Offline Usage Guide

This guide explains how to use bz in air-gapped (disconnected) environments where machines cannot access the internet.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Cache Commands](#cache-commands)
- [Network Modes](#network-modes)
- [Advanced: Registry Mirroring](#advanced-registry-mirroring)
- [Complete Air-gapped Setup](#complete-air-gapped-setup)
- [Troubleshooting](#troubleshooting)
- [CI/CD Integration](#cicd-integration)

---

## Overview

bz provides first-class support for offline and air-gapped environments through:

1. **Local cache** - Download and verify module metadata locally
2. **Offline mode** - Operate entirely from cache without network access
3. **Registry sync** - Mirror BCR to local/internal registries

---

## Quick Start

The simplest approach uses bz's built-in cache commands:

### 1. On a connected machine, download dependencies:

```bash
# Cache all dependencies from your MODULE.bazel
bz cache download

# Or cache specific modules
bz cache download rules_go rules_python gazelle
```

### 2. Verify the cache is complete:

```bash
bz cache verify
```

### 3. Transfer the cache:

```bash
# The cache is at ~/.cache/bz by default
rsync -av ~/.cache/bz/ user@airgap-machine:~/.cache/bz/

# Or use tar/zip for sneakernet
tar -czf bz-cache.tar.gz ~/.cache/bz
```

### 4. On the air-gapped machine, enable offline mode:

```bash
# Option 1: Environment variable
export BZ_OFFLINE=1

# Option 2: Config file (~/.config/bz/config.toml)
# [network]
# mode = "offline"

# Option 3: CLI flag
bz mod list --offline
```

### 5. Verify and use:

```bash
bz cache verify
bz mod list
bz mod outdated
```

---

## Cache Commands

### Download Dependencies

```bash
# Download all deps from current MODULE.bazel (including transitive)
bz cache download

# Download specific modules (resolves transitive deps automatically)
bz cache download rules_go@0.50.1 gazelle

# Download ALL modules from registry (warning: very large!)
bz cache download --all

# Use custom cache directory
bz cache download --cache-dir=/opt/bz-cache
```

### Verify Cache Completeness

```bash
# Check if cache has all deps needed for offline use
bz cache verify

# Output as JSON (for scripting)
bz cache verify --json
```

Example output:

```
Verifying cache for MODULE.bazel dependencies...

  ✓ rules_go@0.50.1
  ✓ bazel_skylib@1.5.0
  ✗ gazelle@0.38.0 (missing)

Cache is INCOMPLETE. Run 'bz cache download' to fix.
```

### Check Cache Statistics

```bash
bz cache stats
```

Output:

```
Cache directory: /home/user/.cache/bz
Total modules:   15
Total versions:  42
Total size:      12.5 MB
Last updated:    2024-01-15 10:30:00
```

### Clear Cache

```bash
# Clear everything (with confirmation)
bz cache clear

# Clear without confirmation
bz cache clear --force

# Clear specific modules
bz cache clear rules_go gazelle
```

---

## Network Modes

bz supports three network modes:

| Mode             | Description                                 | Use Case            |
| ---------------- | ------------------------------------------- | ------------------- |
| `online`         | Always fetch from network (default)         | Normal development  |
| `prefer-offline` | Use cache if available, fallback to network | Speed up builds     |
| `offline`        | Never access network, fail if not cached    | Air-gapped machines |

### Setting Network Mode

**CLI Flag:**

```bash
bz mod list --offline
bz mod outdated --prefer-offline
```

**Environment Variable:**

```bash
export BZ_OFFLINE=1
# or
export BZ_PREFER_OFFLINE=1
```

**Config File:**

```toml
# ~/.config/bz/config.toml
[network]
mode = "offline"
```

Note: `--offline` and `--prefer-offline` are mutually exclusive. If both are set, `--offline` takes precedence.

---

## Advanced: Registry Mirroring

For larger deployments, you may want to create a full registry mirror that can be served over HTTP or filesystem.

### Using bz mod sync

Create a `bz.star` configuration file:

```starlark
# bz-export.star - Export configuration for air-gapped transfer

bcr = registry.http("https://bcr.bazel.build")

# Export to a portable directory
export_dir = registry.file("/mnt/transfer/bazel-registry")

# All modules your air-gapped environment needs
modules = [
    "bazel_skylib",
    "platforms",
    "rules_cc",
    "rules_go",
    "rules_python",
    "rules_rust",
    "gazelle",
    "protobuf",
    # Add all your dependencies here
]

sync.workflow(
    name = "export",
    origin = bcr,
    destination = export_dir,
    modules = modules,
    versions = sync.all(),  # Export all versions for flexibility
    transforms = [
        sync.skip_yanked(),
        # Rewrite URLs to point to internal server
        sync.rewrite_source_urls(
            pattern = "https://github.com/(.*)/archive/",
            replacement = "https://internal-mirror.local/github/$1/archive/",
        ),
        sync.include_patches(),
    ],
)
```

Run the sync:

```bash
bz mod sync --config bz-export.star export
```

**Important:** The `rewrite_source_urls` transform changes where Bazel downloads source archives from. You must also mirror the actual source archives at that location.

### Setting Up the Air-gapped Registry

1. **Transfer the registry:**

   ```bash
   cp -r /mnt/transfer/bazel-registry /opt/bazel/registry
   ```

2. **Configure Bazel to use it:**

   ```bash
   # .bazelrc
   common --registry=file:///opt/bazel/registry

   # Or serve via HTTP and use:
   common --registry=http://internal-server/bazel-registry
   ```

3. **Configure bz to use it:**

   ```toml
   # /etc/bz/config.toml
   [network]
   mode = "offline"
   registry = "file:///opt/bazel/registry"
   ```

---

## Complete Air-gapped Setup

Here's a complete example for an air-gapped deployment:

### Directory Structure

```
/opt/bazel/
├── registry/
│   ├── bazel_registry.json
│   └── modules/
│       ├── rules_go/
│       │   ├── metadata.json
│       │   └── 0.50.1/
│       │       ├── MODULE.bazel
│       │       └── source.json
│       ├── rules_python/
│       └── ...
└── cache/
    └── modules/ (bz cache)
```

### System Configuration

```toml
# /etc/bz/config.toml

[network]
mode = "offline"
registry = "file:///opt/bazel/registry"

[cache]
dir = "/opt/bazel/cache"

[commands]
# Disable commands that require network
disabled = ["registry ping"]
```

### Bazel Configuration

```bash
# /etc/bazel.bazelrc (system-wide) or project .bazelrc

# Use local registry
common --registry=file:///opt/bazel/registry

# Disable network (extra safety)
build --experimental_downloader_config=/etc/bazel-downloader.cfg
```

### Updating the Mirror

When you need to add new modules or update versions:

1. **On connected machine:** Update your module list and run sync

   ```bash
   bz mod sync --config bz-export.star export
   ```

2. **Transfer only changes:** Use rsync for incremental updates

   ```bash
   rsync -av --delete /mnt/transfer/bazel-registry/ /opt/bazel/registry/
   ```

3. **Update the cache:**

   ```bash
   bz cache download
   bz cache verify
   ```

---

## Troubleshooting

### "Module not found in cache"

```bash
# Check what's in the cache
bz cache stats

# Verify against MODULE.bazel
bz cache verify

# Download missing modules
bz cache download
```

### "Cannot connect to registry" in offline mode

This is expected! In offline mode, bz should never attempt network access. If you see this error:

1. Check that offline mode is enabled: `echo $BZ_OFFLINE`
2. Verify config file: `cat ~/.config/bz/config.toml`
3. Ensure the cache is populated: `bz cache verify`

### Checking Network Mode

```bash
# This should fail in offline mode with no cache
bz mod info some_uncached_module --offline

# This should work if cache is populated
bz mod list --offline
```

### Cache Location

Default cache locations by platform:

| Platform | Default Path              |
| -------- | ------------------------- |
| Linux    | `~/.cache/bz`             |
| macOS    | `~/.cache/bz`             |
| Windows  | `%LOCALAPPDATA%\bz\cache` |

Override with `--cache-dir` flag or `BZ_CACHE_DIR` environment variable.

---

## CI/CD Integration

### Air-gapped Runners

For CI/CD pipelines with limited or no internet access:

```yaml
# .github/workflows/build.yml
jobs:
  build:
    runs-on: self-hosted # Air-gapped runner
    env:
      BZ_OFFLINE: "1"
      BZ_CACHE_DIR: "/opt/shared-cache/bz"
    steps:
      - uses: actions/checkout@v4
      - name: Verify cache
        run: bz cache verify
      - name: Build
        run: bazel build //...
```

### Runners with Slow Connections

For runners with internet access but slow connections:

```yaml
jobs:
  build:
    env:
      BZ_PREFER_OFFLINE: "1"
    steps:
      - name: Restore cache
        uses: actions/cache@v4
        with:
          path: ~/.cache/bz
          key: bz-cache-${{ hashFiles('MODULE.bazel') }}
      - name: Build
        run: |
          bz cache download
          bazel build //...
```
