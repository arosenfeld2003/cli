# Import Checkpoint Metadata from Forks

## Overview

Implement an `entire import` command that fetches checkpoint metadata from contributor forks and merges it into the upstream repository's `entire/checkpoints/v1` branch, using the `Entire-Checkpoint` trailer as the stable identifier.

## Problem Statement

When contributors work on forks and submit PRs, their checkpoint metadata is stranded on their fork's `entire/checkpoints/v1` orphan branch. The pre-push hook that pushes this branch only works with direct push access. After a PR is merged (via merge commit, squash, or rebase), the checkpoint metadata remains in the fork and isn't available in the upstream repository.

## Solution Architecture

### Core Concept

The `Entire-Checkpoint: <id>` trailer in commit messages serves as the stable join key between commits and metadata. This trailer:
- Survives all GitHub merge strategies (merge, squash, rebase)
- Is automatically added by prepare-commit-msg hook (manual-commit) or programmatically (auto-commit)
- Links to checkpoint data stored at `<id[:2]>/<id[2:]>/` on the `entire/checkpoints/v1` branch

### Import Mechanism

The import command will:
1. Accept a fork URL or PR number as input
2. Fetch the `entire/checkpoints/v1` branch from the contributor's fork
3. Scan upstream commit history for `Entire-Checkpoint` trailers
4. For each checkpoint ID found in trailers:
   - Check if metadata already exists in upstream's `entire/checkpoints/v1`
   - If not, copy the checkpoint data from fork to upstream
5. Create a new commit on upstream's `entire/checkpoints/v1` with imported data

## Command Interface

### Primary Usage
```bash
# Import from fork URL
entire import --from https://github.com/contributor/fork

# Import from PR number (requires GitHub API)
entire import --pr 123

# Import specific checkpoint IDs
entire import --from https://github.com/contributor/fork --checkpoint-id abc123def456
```

### Options
- `--from <url>`: Fork repository URL to import from
- `--pr <number>`: GitHub PR number (alternative to --from)
- `--checkpoint-id <id>`: Import specific checkpoint ID(s) only
- `--dry-run`: Show what would be imported without making changes
- `--force`: Overwrite existing checkpoint data if conflicts exist
- `--since <date|commit>`: Only scan commits after specified date or commit

## Implementation Details

### Phase 1: Discovery
1. Parse command arguments to determine source (fork URL or PR)
2. If PR number provided:
   - Use GitHub API to find fork URL
   - Determine merge commit in upstream
3. Add fork as temporary git remote
4. Fetch `entire/checkpoints/v1` from fork

### Phase 2: Trailer Scanning
1. Scan upstream commit history for `Entire-Checkpoint` trailers
   - Default: scan last 100 commits or since last import marker
   - With `--since`: scan from specified point
2. Extract unique checkpoint IDs from trailers
3. Build list of IDs to import

### Phase 3: Metadata Matching
1. For each checkpoint ID:
   - Check if `<id[:2]>/<id[2:]>/` exists in fork's `entire/checkpoints/v1`
   - Check if already exists in upstream's `entire/checkpoints/v1`
2. Build import list of missing checkpoints
3. Handle conflicts based on `--force` flag

### Phase 4: Import Execution
1. For each checkpoint to import:
   - Read all files from `<id[:2]>/<id[2:]>/` in fork
   - Write to same path in upstream's `entire/checkpoints/v1` branch
2. Create commit on `entire/checkpoints/v1`:
   - Subject: `Import checkpoints from <fork-owner>/<fork-repo>`
   - Include list of imported checkpoint IDs
   - Add metadata about import source and timestamp

### Phase 5: Cleanup
1. Remove temporary git remote
2. Report import summary to user

## Data Structures

### Import Manifest
```go
type ImportManifest struct {
    Source      string              // Fork URL or PR number
    CheckpointIDs []string          // IDs found in upstream trailers
    ToImport    []CheckpointImport  // Checkpoints to import
    Skipped     []SkippedCheckpoint // Already exists or conflicts
}

type CheckpointImport struct {
    ID          string
    CommitSHA   string   // Upstream commit with this trailer
    Sessions    []string  // Session IDs in this checkpoint
    FileCount   int
}

type SkippedCheckpoint struct {
    ID     string
    Reason string  // "already exists", "conflict", etc.
}
```

## Validation Strategy

### Pre-Import Validation
- Verify fork URL is accessible
- Confirm `entire/checkpoints/v1` branch exists in fork
- Check write permissions to upstream's `entire/checkpoints/v1`
- Validate checkpoint ID format if specific IDs requested

### Import Validation
- Verify checkpoint directory structure matches expected format
- Validate metadata.json files are parseable
- Ensure no corruption in transcript files
- Check file sizes are within reasonable limits

### Post-Import Validation
- Verify imported checkpoints are readable from upstream
- Confirm checkpoint IDs match between trailers and metadata
- Run integrity check on imported session transcripts

## Test Cases

### Integration Tests

1. **Test GitHub Merge Strategies**
   - Create test repository with Entire enabled
   - Create fork with commits containing trailers
   - Test merge commit strategy:
     - PR with regular merge preserves trailer
     - Import finds and transfers metadata
   - Test squash merge strategy:
     - Multiple commits squashed preserve all trailers
     - Import handles multiple IDs in single commit
   - Test rebase merge strategy:
     - Rebased commits preserve trailers
     - Import works with rewritten commit SHAs

2. **Test Import Scenarios**
   - Import with no existing metadata (fresh import)
   - Import with partial overlap (some checkpoints exist)
   - Import with conflicts (different content, same ID)
   - Import with `--force` flag overwrites conflicts
   - Import with `--dry-run` makes no changes

3. **Test Multi-Session Checkpoints**
   - Checkpoint with single session imports correctly
   - Checkpoint with multiple sessions preserves all
   - Session numbering (0/, 1/, 2/) maintained

4. **Test Error Handling**
   - Fork doesn't exist or inaccessible
   - No `entire/checkpoints/v1` branch in fork
   - Malformed checkpoint data in fork
   - Network failure during fetch

### Unit Tests

1. **Trailer Parsing**
   - Extract single trailer from commit
   - Extract multiple trailers from squashed commit
   - Handle commits without trailers
   - Parse various trailer formats

2. **Checkpoint ID Validation**
   - Valid 12-hex-char IDs accepted
   - Invalid IDs rejected
   - ID-to-path conversion (sharding)

3. **Metadata Operations**
   - Read checkpoint from branch
   - Write checkpoint to branch
   - List existing checkpoints
   - Merge checkpoint data

## Edge Cases

1. **Circular Imports**
   - Upstream previously pushed to fork
   - Fork has upstream's checkpoints
   - Solution: Skip checkpoints that originated from upstream

2. **Renamed/Transferred Forks**
   - Fork URL changed after PR
   - Solution: Follow redirects, use PR API data

3. **Large Checkpoints**
   - Very large transcript files
   - Many sessions in one checkpoint
   - Solution: Streaming reads/writes, progress indicators

4. **Concurrent Imports**
   - Multiple maintainers importing simultaneously
   - Solution: Git locking or merge conflict resolution

5. **Orphaned Metadata**
   - Checkpoint exists but no commit has trailer
   - Solution: Report as orphaned, optional cleanup

## Security Considerations

1. **Authentication**
   - Use existing git credentials for private repos
   - GitHub token for PR API access
   - No storage of fork credentials

2. **Validation**
   - Sanitize fork URLs to prevent injection
   - Validate checkpoint data structure
   - Limit file sizes to prevent DoS

3. **Privacy**
   - Only import checkpoints referenced in upstream
   - Don't expose fork's other branches/data
   - Respect .gitignore equivalent for metadata

## Future Enhancements

1. **Batch Import**
   - Import from multiple forks in one operation
   - Useful after merging several PRs

2. **Automated Import**
   - GitHub Action to run import after PR merge
   - Webhook-triggered imports

3. **Selective Import**
   - Filter by date range
   - Filter by file patterns
   - Filter by session characteristics

4. **Import History**
   - Track what was imported when and from where
   - Audit trail for checkpoint provenance

5. **Conflict Resolution UI**
   - Interactive mode for handling conflicts
   - Side-by-side diff of conflicting checkpoints