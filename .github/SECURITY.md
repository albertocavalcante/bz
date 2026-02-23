# Security Policy

Only the latest release receives security updates.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use [GitHub's private vulnerability reporting](https://github.com/albertocavalcante/bz/security/advisories/new) to submit a report.

You should receive an acknowledgment within 48 hours. From there we'll confirm the issue, coordinate a fix, and agree on a disclosure timeline.

## What Counts as a Security Issue

As a CLI tool that interacts with registries and processes dependency metadata, the following are in scope:

- Command injection or arbitrary code execution
- Path traversal when writing files (SBOM output, cache, etc.)
- Credential leakage (registry auth tokens in logs, error messages, or SBOM output)
- TOCTOU or symlink attacks in cache operations
- Dependency confusion or registry spoofing
- Vulnerabilities in direct dependencies of `bz` itself

The following are **not** in scope:

- Vulnerabilities in Bazel modules that `bz audit` reports on (those belong to the upstream module)
- Denial of service via large MODULE.bazel files (expected local input)

## Disclosure Policy

We follow coordinated disclosure. Once a fix is available, we will:

1. Release a patched version
2. Publish a GitHub Security Advisory
3. Credit the reporter (unless they prefer to remain anonymous)
