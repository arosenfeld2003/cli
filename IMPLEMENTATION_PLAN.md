# Implementation Plan

## Current Focus
- Implement `entire import` command for checkpoint metadata import from contributor forks

## Tasks

### Phase 1: Core Import Command (Priority: HIGH)
- [x] Create `cmd/entire/cli/import.go` with basic command structure
- [x] Add command registration to `cmd/entire/cli/root.go`
- [x] Implement `--from <fork-url>` flag for direct fork import
- [x] Implement `--checkpoint-id <id>` flag for specific checkpoint import
- [x] Implement `--dry-run` flag to preview import actions

### Phase 2: Fork Remote Management (Priority: HIGH)
- [x] Add temporary git remote for fork repository (basic implementation)
- [x] Fetch `entire/checkpoints/v1` branch from fork (basic implementation)
- [x] Implement remote cleanup after import
- [ ] Handle authentication for private repositories (needs improvement)

### Phase 3: Trailer Scanning & Matching (Priority: HIGH)
- [x] Scan upstream commits for `Entire-Checkpoint` trailers (implemented)
- [x] Match checkpoint IDs between upstream and fork (basic implementation)
- [ ] Build complete import manifest with ToImport and Skipped lists
- [ ] Validate checkpoint data structure from fork

### Phase 4: Metadata Import Logic (Priority: HIGH)
- [ ] Copy checkpoint data from fork to upstream
- [ ] Handle multi-session checkpoints (numbered subfolders)
- [ ] Create import commit on `entire/checkpoints/v1` branch
- [ ] Add import summary and reporting

### Phase 5: GitHub PR Integration (Priority: MEDIUM)
- [ ] Add GitHub API client dependency to `go.mod`
- [ ] Implement `--pr <number>` flag for PR-based import
- [ ] Fetch fork URL from PR metadata
- [ ] Handle renamed/transferred forks

### Phase 6: Advanced Features (Priority: LOW)
- [ ] Implement `--force` flag to overwrite existing checkpoints
- [ ] Implement `--since <date|commit>` for partial imports
- [ ] Add progress indicators for large imports
- [ ] Implement concurrent import protection

### Phase 7: Testing (Priority: HIGH)
- [ ] Unit tests for trailer scanning
- [ ] Unit tests for checkpoint matching
- [ ] Integration tests for import workflow
- [ ] Tests for GitHub merge strategies (merge, squash, rebase)
- [ ] Error handling tests

## Completed
- Basic `entire import` command structure and compilation
- Core command flags and argument parsing
- Basic git remote management for fork repositories
- Initial trailer scanning and checkpoint ID matching

**Note**: The basic structure is in place and compiles, but the actual import logic needs to be completed.

## Notes
- Leverage existing trailer parsing from `cmd/entire/cli/trailers/trailers.go`
- Use existing checkpoint storage from `cmd/entire/cli/checkpoint/committed.go`
- Follow command patterns from `explain.go` and `rewind.go`
- Start with basic `--from` URL import before adding GitHub API integration