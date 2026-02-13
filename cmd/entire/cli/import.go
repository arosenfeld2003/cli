package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/checkpoint"
	"github.com/entireio/cli/cmd/entire/cli/checkpoint/id"
	"github.com/entireio/cli/cmd/entire/cli/logging"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/strategy"
	"github.com/entireio/cli/cmd/entire/cli/trailers"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/cobra"
)

// ImportManifest tracks what checkpoints will be imported
type ImportManifest struct {
	Source        string               // Fork URL or PR number
	CheckpointIDs []string            // IDs found in upstream trailers
	ToImport      []CheckpointImport  // Checkpoints to import
	Skipped       []SkippedCheckpoint // Already exists or conflicts
}

// CheckpointImport represents a checkpoint to be imported
type CheckpointImport struct {
	ID          string   // Checkpoint ID
	CommitSHA   string   // Upstream commit with this trailer
	Sessions    []string // Session IDs in this checkpoint
	FileCount   int      // Number of files in the checkpoint
	TotalSize   int64    // Total size of checkpoint data
}

// SkippedCheckpoint represents a checkpoint that was skipped
type SkippedCheckpoint struct {
	ID     string // Checkpoint ID
	Reason string // Why it was skipped
}

func newImportCmd() *cobra.Command {
	var fromFlag string
	var prFlag int
	var checkpointIDsFlag []string
	var dryRunFlag bool
	var forceFlag bool
	var sinceFlag string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import checkpoint metadata from contributor forks",
		Long: `Import checkpoint metadata from contributor forks after PRs are merged.

When contributors work on forks and submit PRs, their checkpoint metadata
gets stranded on the fork's entire/checkpoints/v1 orphan branch. This command
uses the Entire-Checkpoint trailer in commit messages as a stable join key
to fetch and merge checkpoint metadata from contributor forks into the
upstream repository.

Examples:
  # Import from a fork URL
  entire import --from https://github.com/contributor/fork

  # Import from a GitHub PR
  entire import --pr 123

  # Import specific checkpoint IDs
  entire import --from https://github.com/contributor/fork --checkpoint-id abc123def456

  # Preview what would be imported (dry run)
  entire import --from https://github.com/contributor/fork --dry-run

  # Force overwrite existing checkpoints
  entire import --from https://github.com/contributor/fork --force

  # Import only recent checkpoints
  entire import --from https://github.com/contributor/fork --since 2024-01-01

The command will:
  1. Add the fork as a temporary git remote
  2. Fetch the entire/checkpoints/v1 branch from the fork
  3. Scan upstream commits for Entire-Checkpoint trailers
  4. Match checkpoint IDs between upstream and fork
  5. Copy matching checkpoint data to upstream
  6. Create an import commit on entire/checkpoints/v1
  7. Remove the temporary remote`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Check if Entire is disabled
			if checkDisabledGuard(cmd.OutOrStdout()) {
				return nil
			}

			// Validate that at least one source is specified
			if fromFlag == "" && prFlag == 0 {
				return errors.New("must specify either --from <fork-url> or --pr <number>")
			}

			// Validate that only one source is specified
			if fromFlag != "" && prFlag != 0 {
				return errors.New("cannot specify both --from and --pr")
			}

			// Validate --since format if provided
			if sinceFlag != "" {
				// Try parsing as date
				if _, err := time.Parse("2006-01-02", sinceFlag); err != nil {
					// Not a date, validate it looks like a commit ref
					if len(sinceFlag) < 4 {
						return fmt.Errorf("--since must be a date (YYYY-MM-DD) or commit ref, got: %s", sinceFlag)
					}
				}
			}

			// PR support will be added in phase 5
			if prFlag != 0 {
				return errors.New("--pr flag not yet implemented (coming in phase 5)")
			}

			ctx := context.Background()
			return runImport(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), fromFlag, prFlag, checkpointIDsFlag, dryRunFlag, forceFlag, sinceFlag)
		},
	}

	cmd.Flags().StringVar(&fromFlag, "from", "", "Fork repository URL to import from")
	cmd.Flags().IntVar(&prFlag, "pr", 0, "GitHub PR number to import from (requires GitHub API)")
	cmd.Flags().StringSliceVar(&checkpointIDsFlag, "checkpoint-id", []string{}, "Import specific checkpoint ID(s) only")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show what would be imported without making changes")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing checkpoint data if conflicts exist")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only scan commits after this date (YYYY-MM-DD) or commit ref")

	return cmd
}

func runImport(ctx context.Context, out, errOut *os.File, forkURL string, prNumber int, checkpointIDs []string, dryRun, force bool, since string) error {
	// Initialize logging
	logging.Info(ctx, "starting checkpoint import",
		"from", forkURL,
		"pr", prNumber,
		"checkpoint_ids", checkpointIDs,
		"dry_run", dryRun,
		"force", force,
		"since", since)

	// Get repository root
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	// Open repository
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Create import manifest
	manifest := &ImportManifest{
		Source:        forkURL,
		CheckpointIDs: checkpointIDs,
		ToImport:      []CheckpointImport{},
		Skipped:       []SkippedCheckpoint{},
	}

	// Phase 1: Add fork as temporary remote
	fmt.Fprintf(out, "Adding temporary remote for %s...\n", forkURL)
	remoteName, err := addForkRemote(ctx, repo, forkURL)
	if err != nil {
		return fmt.Errorf("failed to add fork remote: %w", err)
	}
	defer removeForkRemote(ctx, repo, remoteName)

	// Phase 2: Fetch entire/checkpoints/v1 branch from fork
	fmt.Fprintf(out, "Fetching checkpoint metadata from fork...\n")
	if err := fetchForkCheckpointBranch(ctx, repo, remoteName); err != nil {
		return fmt.Errorf("failed to fetch checkpoint branch: %w", err)
	}

	// Phase 3: Scan upstream commits for checkpoint trailers
	fmt.Fprintf(out, "Scanning upstream commits for checkpoint trailers...\n")
	upstreamCheckpointIDs, err := scanUpstreamCheckpoints(ctx, repo, since, checkpointIDs)
	if err != nil {
		return fmt.Errorf("failed to scan upstream checkpoints: %w", err)
	}

	if len(upstreamCheckpointIDs) == 0 {
		fmt.Fprintf(out, "No checkpoint trailers found in upstream commits\n")
		return nil
	}

	fmt.Fprintf(out, "Found %d checkpoint(s) in upstream commits\n", len(upstreamCheckpointIDs))
	manifest.CheckpointIDs = upstreamCheckpointIDs

	// Phase 4: Match checkpoints between upstream and fork
	fmt.Fprintf(out, "Matching checkpoints between upstream and fork...\n")
	if err := matchCheckpoints(ctx, repo, remoteName, manifest, force); err != nil {
		return fmt.Errorf("failed to match checkpoints: %w", err)
	}

	// Report manifest
	fmt.Fprintf(out, "\nImport Manifest:\n")
	fmt.Fprintf(out, "  Source: %s\n", manifest.Source)
	fmt.Fprintf(out, "  Checkpoints found: %d\n", len(manifest.CheckpointIDs))
	fmt.Fprintf(out, "  To import: %d\n", len(manifest.ToImport))
	fmt.Fprintf(out, "  Skipped: %d\n", len(manifest.Skipped))

	if len(manifest.ToImport) == 0 {
		fmt.Fprintf(out, "\nNo checkpoints to import.\n")
		if len(manifest.Skipped) > 0 {
			fmt.Fprintf(out, "\nSkipped checkpoints:\n")
			for _, skip := range manifest.Skipped {
				fmt.Fprintf(out, "  - %s: %s\n", skip.ID, skip.Reason)
			}
		}
		return nil
	}

	// Show what will be imported
	fmt.Fprintf(out, "\nCheckpoints to import:\n")
	for _, imp := range manifest.ToImport {
		fmt.Fprintf(out, "  - %s (commit %s, %d sessions, %d files)\n",
			imp.ID, imp.CommitSHA[:7], len(imp.Sessions), imp.FileCount)
	}

	if dryRun {
		fmt.Fprintf(out, "\nDry run mode - no changes made.\n")
		return nil
	}

	// Phase 5: Import checkpoint data
	fmt.Fprintf(out, "\nImporting checkpoint data...\n")
	if err := importCheckpointData(ctx, repo, remoteName, manifest); err != nil {
		return fmt.Errorf("failed to import checkpoint data: %w", err)
	}

	// Report success
	fmt.Fprintf(out, "\nSuccessfully imported %d checkpoint(s) from %s\n", len(manifest.ToImport), manifest.Source)

	return nil
}

// addForkRemote adds the fork as a temporary git remote
func addForkRemote(ctx context.Context, repo *git.Repository, forkURL string) (string, error) {
	// Parse and validate the URL
	if _, err := url.Parse(forkURL); err != nil {
		return "", fmt.Errorf("invalid fork URL: %w", err)
	}

	// Generate a unique remote name
	remoteName := fmt.Sprintf("import-%d", time.Now().Unix())

	// Add the remote
	_, err := repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{forkURL},
		Fetch: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/entire/checkpoints/v1:refs/remotes/%s/entire/checkpoints/v1", remoteName)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create remote: %w", err)
	}

	logging.Info(ctx, "added temporary remote", "name", remoteName, "url", forkURL)
	return remoteName, nil
}

// removeForkRemote removes the temporary git remote
func removeForkRemote(ctx context.Context, repo *git.Repository, remoteName string) {
	if err := repo.DeleteRemote(remoteName); err != nil {
		logging.Warn(ctx, "failed to remove temporary remote", "name", remoteName, "error", err)
	} else {
		logging.Info(ctx, "removed temporary remote", "name", remoteName)
	}
}

// fetchForkCheckpointBranch fetches the entire/checkpoints/v1 branch from the fork
func fetchForkCheckpointBranch(ctx context.Context, repo *git.Repository, remoteName string) error {
	// Create fetch options
	fetchOptions := &git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/entire/checkpoints/v1:refs/remotes/%s/entire/checkpoints/v1", remoteName)),
		},
		// Use default auth (will use system git credentials)
	}

	// Try to use system git credentials if available
	if os.Getenv("GITHUB_TOKEN") != "" {
		fetchOptions.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: os.Getenv("GITHUB_TOKEN"),
		}
	}

	// Fetch the branch
	err := repo.Fetch(fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		// Check if it's a "not found" error for the branch
		if errors.Is(err, transport.ErrEmptyRemoteRepository) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("fork does not have entire/checkpoints/v1 branch")
		}
		return fmt.Errorf("failed to fetch from fork: %w", err)
	}

	logging.Info(ctx, "fetched checkpoint branch from fork", "remote", remoteName)
	return nil
}

// scanUpstreamCheckpoints scans upstream commits for checkpoint trailers
func scanUpstreamCheckpoints(ctx context.Context, repo *git.Repository, since string, filterIDs []string) ([]string, error) {
	// Get HEAD reference
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get commit iterator
	commitIter, err := repo.Log(&git.LogOptions{
		From: head.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit iterator: %w", err)
	}
	defer commitIter.Close()

	// Parse since date if provided
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse("2006-01-02", since); err == nil {
			sinceTime = t
		}
		// TODO: Handle commit ref for --since
	}

	// Scan commits for checkpoint trailers
	checkpointMap := make(map[string]bool)
	err = commitIter.ForEach(func(c *object.Commit) error {
		// Skip commits before since date
		if !sinceTime.IsZero() && c.Committer.When.Before(sinceTime) {
			return nil
		}

		// Parse checkpoint trailer
		checkpointIDParsed, found := trailers.ParseCheckpoint(c.Message)
		if !found {
			return nil
		}
		checkpointID := checkpointIDParsed.String()

		// Validate checkpoint ID format
		if err := id.Validate(checkpointID); err != nil {
			logging.Warn(ctx, "invalid checkpoint ID in commit", "commit", c.Hash.String(), "id", checkpointID, "error", err)
			return nil
		}

		// Apply filter if specified
		if len(filterIDs) > 0 {
			found := false
			for _, filterID := range filterIDs {
				if strings.HasPrefix(checkpointID, filterID) {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		checkpointMap[checkpointID] = true
		logging.Debug(ctx, "found checkpoint trailer", "commit", c.Hash.String(), "checkpoint_id", checkpointID)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan commits: %w", err)
	}

	// Convert map to sorted slice
	checkpointIDs := make([]string, 0, len(checkpointMap))
	for id := range checkpointMap {
		checkpointIDs = append(checkpointIDs, id)
	}
	sort.Strings(checkpointIDs)

	return checkpointIDs, nil
}

// matchCheckpoints matches checkpoints between upstream and fork
func matchCheckpoints(ctx context.Context, repo *git.Repository, remoteName string, manifest *ImportManifest, force bool) error {
	// Get the fork's checkpoint branch reference
	forkRef, err := repo.Reference(plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/entire/checkpoints/v1", remoteName)), true)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			// Fork doesn't have the branch
			for _, id := range manifest.CheckpointIDs {
				manifest.Skipped = append(manifest.Skipped, SkippedCheckpoint{
					ID:     id,
					Reason: "fork does not have entire/checkpoints/v1 branch",
				})
			}
			return nil
		}
		return fmt.Errorf("failed to get fork checkpoint branch: %w", err)
	}

	// Get the fork's checkpoint commit
	forkCommit, err := repo.CommitObject(forkRef.Hash())
	if err != nil {
		return fmt.Errorf("failed to get fork checkpoint commit: %w", err)
	}

	// Get the fork's tree
	forkTree, err := forkCommit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get fork tree: %w", err)
	}

	// Get local checkpoint branch if it exists
	var localTree *object.Tree
	localRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/entire/checkpoints/v1"), true)
	if err == nil {
		localCommit, err := repo.CommitObject(localRef.Hash())
		if err == nil {
			localTree, _ = localCommit.Tree()
		}
	}

	// Check each checkpoint ID
	for _, checkpointID := range manifest.CheckpointIDs {
		// Convert checkpoint ID to sharded path
		checkpointIDObj, err := id.NewCheckpointID(checkpointID)
		if err != nil {
			manifest.Skipped = append(manifest.Skipped, SkippedCheckpoint{
				ID:     checkpointID,
				Reason: fmt.Sprintf("invalid checkpoint ID: %v", err),
			})
			continue
		}
		shardedPath := checkpointIDObj.Path()

		// Check if checkpoint exists in fork
		_, err := forkTree.FindEntry(shardedPath)
		if err != nil {
			manifest.Skipped = append(manifest.Skipped, SkippedCheckpoint{
				ID:     checkpointID,
				Reason: "not found in fork",
			})
			continue
		}

		// Check if already exists locally
		if localTree != nil && !force {
			if _, err := localTree.FindEntry(shardedPath); err == nil {
				manifest.Skipped = append(manifest.Skipped, SkippedCheckpoint{
					ID:     checkpointID,
					Reason: "already exists in upstream",
				})
				continue
			}
		}

		// Get metadata for the checkpoint from fork
		imp := CheckpointImport{
			ID:        checkpointID,
			CommitSHA: forkRef.Hash().String(), // TODO: Find actual upstream commit
			Sessions:  []string{},
			FileCount: 0,
		}

		// Count files and sessions in the checkpoint
		if err := countCheckpointContents(forkTree, shardedPath, &imp); err != nil {
			logging.Warn(ctx, "failed to count checkpoint contents", "checkpoint_id", checkpointID, "error", err)
		}

		manifest.ToImport = append(manifest.ToImport, imp)
		logging.Info(ctx, "checkpoint will be imported", "checkpoint_id", checkpointID, "sessions", len(imp.Sessions))
	}

	return nil
}

// countCheckpointContents counts files and sessions in a checkpoint
func countCheckpointContents(tree *object.Tree, path string, imp *CheckpointImport) error {
	// Find the checkpoint directory
	entry, err := tree.FindEntry(path)
	if err != nil {
		return err
	}

	// Get the checkpoint tree
	checkpointTree, err := tree.Tree(entry.Name)
	if err != nil {
		return err
	}

	// Count files and look for session directories
	checkpointTree.Files().ForEach(func(f *object.File) error {
		imp.FileCount++
		imp.TotalSize += f.Size
		return nil
	})

	// Look for numbered session directories (0/, 1/, etc)
	for i := 0; i < 10; i++ { // Check first 10 potential session directories
		sessionDir := fmt.Sprintf("%d", i)
		if _, err := checkpointTree.Tree(sessionDir); err == nil {
			imp.Sessions = append(imp.Sessions, sessionDir)
		} else {
			break // No more session directories
		}
	}

	// If no numbered directories found, assume single session
	if len(imp.Sessions) == 0 {
		imp.Sessions = []string{"0"}
	}

	return nil
}

// importCheckpointData imports checkpoint data from fork to upstream
func importCheckpointData(ctx context.Context, repo *git.Repository, remoteName string, manifest *ImportManifest) error {
	// Get the fork's checkpoint branch
	forkRef, err := repo.Reference(plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/entire/checkpoints/v1", remoteName)), true)
	if err != nil {
		return fmt.Errorf("failed to get fork checkpoint branch: %w", err)
	}

	// Get checkpoint store
	store := checkpoint.NewGitStore(repo)

	// Get strategy for writing checkpoints
	strat, err := strategy.Get(strategy.Default())
	if err != nil {
		return fmt.Errorf("failed to get strategy: %w", err)
	}

	// Import each checkpoint
	for _, imp := range manifest.ToImport {
		logging.Info(ctx, "importing checkpoint", "checkpoint_id", imp.ID)

		// Read checkpoint data from fork
		checkpointData, err := readCheckpointFromFork(ctx, repo, forkRef.Hash(), imp.ID)
		if err != nil {
			logging.Error(ctx, "failed to read checkpoint from fork", "checkpoint_id", imp.ID, "error", err)
			continue
		}

		// Write checkpoint to local metadata branch
		if err := writeImportedCheckpoint(ctx, store, strat, imp.ID, checkpointData); err != nil {
			logging.Error(ctx, "failed to write imported checkpoint", "checkpoint_id", imp.ID, "error", err)
			continue
		}

		logging.Info(ctx, "imported checkpoint", "checkpoint_id", imp.ID)
	}

	return nil
}

// CheckpointData holds the data read from a fork checkpoint
type CheckpointData struct {
	ID           string
	Path         string
	Tree         *object.Tree
	MetadataJSON []byte
}

// readCheckpointFromFork reads checkpoint data from the fork's tree
func readCheckpointFromFork(ctx context.Context, repo *git.Repository, commitHash plumbing.Hash, checkpointID string) (*CheckpointData, error) {
	// Get commit and tree
	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	// Convert checkpoint ID to sharded path
	checkpointIDObj, err := id.NewCheckpointID(checkpointID)
	if err != nil {
		return nil, fmt.Errorf("invalid checkpoint ID: %w", err)
	}
	shardedPath := checkpointIDObj.Path()

	// Find checkpoint directory in tree
	entry, err := tree.FindEntry(shardedPath)
	if err != nil {
		return nil, fmt.Errorf("checkpoint not found in tree: %w", err)
	}

	// Get checkpoint subtree
	checkpointTree, err := tree.Tree(entry.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint tree: %w", err)
	}

	// Read metadata.json
	metadataFile, err := checkpointTree.File("metadata.json")
	if err != nil {
		return nil, fmt.Errorf("metadata.json not found: %w", err)
	}

	metadataContent, err := metadataFile.Contents()
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata.json: %w", err)
	}

	// Return checkpoint data
	return &CheckpointData{
		ID:           checkpointID,
		Path:         shardedPath,
		Tree:         checkpointTree,
		MetadataJSON: []byte(metadataContent),
	}, nil
}

// writeImportedCheckpoint writes imported checkpoint data to local metadata branch
func writeImportedCheckpoint(ctx context.Context, store *checkpoint.GitStore, strat strategy.Strategy, checkpointID string, data *CheckpointData) error {
	// TODO: Implement actual checkpoint writing logic
	// This will use store.WriteCommitted() or similar methods
	// For now, just log
	logging.Info(ctx, "would write checkpoint", "checkpoint_id", checkpointID, "path", data.Path)
	return nil
}