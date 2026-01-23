# bz - CLI for Bzlmod

set shell := ["bash", "-uc"]

# Default recipe
default: help

# Build the binary
build:
    go build -o bz .

# Build with version info
build-release version="dev":
    go build -ldflags "-s -w -X main.version={{version}}" -o bz .

# Run tests
test:
    go test -race ./...

# Run linters
lint:
    golangci-lint run --timeout=5m

# Format code
fmt:
    go fmt ./...
    dprint fmt

# Check formatting
check:
    test -z "$(gofmt -l .)"
    dprint check

# Tidy dependencies
tidy:
    go mod tidy

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

# Clean build artifacts
clean:
    rm -f bz
    go clean

# Run the binary
run *args:
    go run . {{args}}

# Watch and rebuild on changes (requires watchexec)
watch:
    watchexec -e go -r -- just build

# Show help
help:
    @just --list
