## Project Goal

Implement a checkpoint metadata import mechanism to transfer Entire session metadata from contributor forks to upstream repositories after PRs are merged.

## Tech Stack

- Language: Go 1.25.x
- Build tool: mise, go modules
- CLI framework: github.com/spf13/cobra, github.com/charmbracelet/huh
- Git operations: github.com/go-git/go-git/v5, git CLI for certain operations
- Testing: Go standard testing with parallelization

## Build & Run

- Build: `mise run build` or `go build ./cmd/entire`
- Run: `./entire`

## Validation

- Tests: `mise run test:ci` (unit + integration tests)
- Typecheck: `go build -o /dev/null ./...`
- Lint: `mise run lint` (golangci-lint)
- Formatting: `mise run fmt` (gofmt)

## Operational Notes

### Key Context

The Entire CLI uses checkpoint IDs (12-hex-char) in git commit trailers as stable identifiers that survive GitHub merge operations (merge, squash, rebase). These trailers link user commits to metadata stored on the `entire/checkpoints/v1` orphan branch.

### Codebase Patterns

- Command implementations: `cmd/entire/cli/commands/`
- Strategy implementations: `cmd/entire/cli/strategy/`
- Checkpoint storage: `cmd/entire/cli/checkpoint/`
- Session management: `cmd/entire/cli/session/`
- Always use `t.Parallel()` in tests unless modifying process-global state
- Error handling: Use `SilentError` for custom user-friendly messages
- Settings access through `cmd/entire/cli/settings/` package