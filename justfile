# bz - Bazel module management CLI

set shell := ["bash", "-uc"]

# ─────────────────────────────────────────────────────────────────────────────
# Default
# ─────────────────────────────────────────────────────────────────────────────

# Default: show available recipes
default:
    @just --list

# ─────────────────────────────────────────────────────────────────────────────
# Build & Test
# ─────────────────────────────────────────────────────────────────────────────

# Build the binary
build:
    go build -o bz .

# Build with version info
build-release version="dev":
    go build -ldflags "-s -w -X main.version={{version}}" -o bz .

# Run tests
test:
    go test -race ./...

# Run tests with coverage
test-coverage:
    go test -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
    go test -bench=. -benchmem ./...

# ─────────────────────────────────────────────────────────────────────────────
# Linting & Formatting
# ─────────────────────────────────────────────────────────────────────────────

# Run linters
lint:
    golangci-lint run --timeout=5m

# Lint and fix
lint-fix:
    golangci-lint run --fix --timeout=5m

# Format code
fmt:
    go fmt ./...
    gofumpt -w .

# Check formatting (CI)
fmt-check:
    test -z "$(gofmt -l .)"

# Lint GitHub Actions
lint-actions:
    actionlint

# ─────────────────────────────────────────────────────────────────────────────
# Dependencies
# ─────────────────────────────────────────────────────────────────────────────

# Tidy dependencies
tidy:
    go mod tidy

# Download dependencies
deps:
    go mod download

# Verify dependencies
verify:
    go mod verify

# ─────────────────────────────────────────────────────────────────────────────
# Install
# ─────────────────────────────────────────────────────────────────────────────

# Install to ~/.local/bin (atomic)
install: build
    #!/usr/bin/env bash
    set -euo pipefail
    dest="${HOME}/.local/bin/bz"
    mkdir -p "$(dirname "$dest")"
    tmp=$(mktemp)
    cp bz "$tmp"
    chmod 755 "$tmp"
    mv "$tmp" "$dest"
    echo "Installed to $dest"

# Install to specific path (atomic)
install-to path: build
    #!/usr/bin/env bash
    set -euo pipefail
    dest="{{path}}"
    mkdir -p "$(dirname "$dest")"
    tmp=$(mktemp)
    cp bz "$tmp"
    chmod 755 "$tmp"
    mv "$tmp" "$dest"
    echo "Installed to $dest"

# Install to GOPATH/bin (atomic)
install-go: build
    #!/usr/bin/env bash
    set -euo pipefail
    dest="${GOPATH:-$HOME/go}/bin/bz"
    mkdir -p "$(dirname "$dest")"
    tmp=$(mktemp)
    cp bz "$tmp"
    chmod 755 "$tmp"
    mv "$tmp" "$dest"
    echo "Installed to $dest"

# ─────────────────────────────────────────────────────────────────────────────
# Documentation
# ─────────────────────────────────────────────────────────────────────────────

# Install docs dependencies
docs-install:
    cd docs && bun install

# Build docs for production
docs-build:
    cd docs && bun run build

# Serve docs locally (dev mode with hot reload)
docs-dev:
    cd docs && bun run dev

# Preview production build locally
docs-preview:
    cd docs && bun run preview

# Build and serve docs (production build)
docs: docs-build docs-preview

# Clean docs build artifacts
docs-clean:
    rm -rf docs/dist docs/.astro docs/node_modules

# ─────────────────────────────────────────────────────────────────────────────
# Development
# ─────────────────────────────────────────────────────────────────────────────

# Clean build artifacts
clean:
    rm -f bz coverage.out coverage.html
    go clean

# Run the binary
run *args:
    go run . {{args}}

# Watch and rebuild on changes (requires watchexec)
watch:
    watchexec -e go -r -- just build

# Run all checks (CI simulation)
check: fmt-check lint test

# ─────────────────────────────────────────────────────────────────────────────
# Open in Browser / Editor
# ─────────────────────────────────────────────────────────────────────────────

# Open repository in browser
open:
    open https://github.com/albertocavalcante/bz

# Open GitHub Actions
actions:
    open https://github.com/albertocavalcante/bz/actions

# Open GitHub Issues
issues:
    open https://github.com/albertocavalcante/bz/issues

# Open Pull Requests
prs:
    open https://github.com/albertocavalcante/bz/pulls

# Open Releases
releases:
    open https://github.com/albertocavalcante/bz/releases

# Open docs in browser
docs-open:
    open https://albertocavalcante.github.io/bz/

# Open in VSCode
code:
    code .

# Open in Cursor
cursor:
    cursor .

# Open in Zed
zed:
    zed .

# ─────────────────────────────────────────────────────────────────────────────
# CI / Workflows
# ─────────────────────────────────────────────────────────────────────────────

# Watch CI status
ci-watch:
    gh run watch

# List recent workflow runs
ci-list:
    gh run list --limit 10

# Trigger nightly build
nightly:
    gh workflow run nightly.yml --field force=true

# Trigger docs deployment manually
docs-deploy:
    gh workflow run docs.yml

# ─────────────────────────────────────────────────────────────────────────────
# Release
# ─────────────────────────────────────────────────────────────────────────────

# Create a new release tag
release version:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! "{{version}}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
        echo "Error: Version must be in format v1.2.3 or v1.2.3-rc1"
        exit 1
    fi
    git tag -a "{{version}}" -m "Release {{version}}"
    echo "Created tag {{version}}"
    echo "Run 'git push origin {{version}}' to trigger release workflow"

# List tags
tags:
    git tag -l --sort=-v:refname | head -10
