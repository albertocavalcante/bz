# Agent Guidelines

Guidelines for AI agents and contributors working on this project.

## Issue References and Replies

- Use `Refs #N` for work on an issue and `Related to #N` for context in PR
  descriptions, commits, and squash messages. Treat Development sidebar links
  as potential closing links. A closing link requires explicit maintainer
  approval for that PR and issue.
- Before closing a human-filed issue, a maintainer must post a substantive
  reply on the issue and record its permalink. Keep the issue open while
  work needed to resolve the report is pending.
- Put a visible AI disclosure in every issue comment whose wording or
  substance AI generated or rewrote. Agents need maintainer approval to post.

## Documentation

- Update the existing documentation that owns the topic. Add a Markdown file
  only when it serves a distinct audience or task that the existing docs
  cannot serve well.

## Commit Messages

This project follows [Semantic Commit Messages](https://www.conventionalcommits.org/).

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type       | Description                                |
| ---------- | ------------------------------------------ |
| `feat`     | New feature                                |
| `fix`      | Bug fix                                    |
| `docs`     | Documentation changes                      |
| `chore`    | Maintenance, dependencies, tooling         |
| `refactor` | Code restructuring without behavior change |
| `test`     | Adding or updating tests                   |
| `ci`       | CI/CD changes                              |
| `perf`     | Performance improvements                   |
| `style`    | Code style (formatting, whitespace)        |
| `build`    | Build system changes                       |

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
