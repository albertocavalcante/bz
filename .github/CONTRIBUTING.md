# Contributing to bz

This is a quick-start guide. See the [full contributing guide](https://albertocavalcante.github.io/bz/contributing/) for details.

## Quick Start

```bash
# Fork and clone
git clone https://github.com/YOUR-USERNAME/bz.git
cd bz

# Build
go build -o bz .

# Run tests
go test ./...

# Run linter
go tool -modfile=tools.go.mod golangci-lint run --config=tools/lint/golangci.toml
```

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation
- `refactor:` — Code refactoring
- `test:` — Tests
- `chore:` — Maintenance

## Submitting a PR

1. Create a branch from `main`
2. Make your changes and add tests
3. Ensure `go test ./...` passes
4. Open a Pull Request

## Need Help?

- [Open a Discussion](https://github.com/albertocavalcante/bz/discussions) for questions
- [Open an Issue](https://github.com/albertocavalcante/bz/issues) for bugs or feature requests
