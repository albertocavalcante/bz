# Agent Guidelines

Guidelines for AI agents and contributors working on this project.

## Commit Messages

This project follows [Semantic Commit Messages](https://www.conventionalcommits.org/).

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type       | Description                                      |
| ---------- | ------------------------------------------------ |
| `feat`     | New feature                                      |
| `fix`      | Bug fix                                          |
| `docs`     | Documentation changes                            |
| `chore`    | Maintenance, dependencies, tooling               |
| `refactor` | Code restructuring without behavior change       |
| `test`     | Adding or updating tests                         |
| `ci`       | CI/CD changes                                    |
| `perf`     | Performance improvements                         |
| `style`    | Code style (formatting, whitespace)              |
| `build`    | Build system changes                             |

### Examples

```
feat: add list command for MODULE.bazel deps
feat(cli): add --outdated flag to list command
fix(parser): handle empty MODULE.bazel files
docs: update README with usage examples
chore: add lefthook for git hooks
refactor(registry): extract HTTP client config
test: add integration tests for BCR client
```

### Validation

Commit messages are validated by lefthook. Install hooks with:

```bash
lefthook install
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused and small
- Add comments for exported functions
- Handle errors explicitly

## Testing

Run tests before committing:

```bash
go test ./...
```

## Dependencies

- Use `go mod tidy` after adding/removing dependencies
- Prefer standard library when possible
- Document why external dependencies are needed
