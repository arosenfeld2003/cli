# Agents Operational Guide - Entire CLI

## Running the Application

### Prerequisites
- Go 1.25.x runtime
- mise build tool (`curl -fsSL https://mise.run | sh`)

### Build Commands
```bash
# Install dependencies
go mod download

# Format code (required before commit)
mise run fmt

# Run linter (required before commit)
mise run lint

# Run unit tests
mise run test

# Run integration tests
mise run test:integration

# Run all tests (CI equivalent)
mise run test:ci

# Check for code duplication
mise run dup
```

### Development Workflow
1. Make changes to code
2. Run `mise run fmt` to format
3. Run `mise run lint` to check for issues
4. Run `mise run test:ci` to verify all tests pass
5. Commit with descriptive message

### Key Commands
- `entire enable` - Enable Entire in a git repository
- `entire status` - Show current session status
- `entire rewind` - Restore to a previous checkpoint
- `entire explain` - Explain checkpoint history

### Environment Variables
- `ACCESSIBLE=1` - Enable accessibility mode for screen readers
- `ENTIRE_DEBUG=1` - Enable debug logging