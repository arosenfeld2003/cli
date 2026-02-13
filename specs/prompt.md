# Problem
We need to get checkpoint metadata from a contributor's fork to upstream!                                                                                                                                                                 

# Current Architecture
`entire/checkpoints/v1` is an orphan branch that gets pushed via the pre-push hook — 
but that only works with direct push access. 
For fork-based contributions, the metadata is stranded.                                                                                                                                                                           
                                                                                                                                                                                                  
# Mitigation Strategy
 - Use the Entire-Checkpoint trailer as the sole stable link, paired with an import mechanism.
### Core Concept:
  - The Entire-Checkpoint: <id> trailer survives GitHub squash-merge (trailers concatenated into squashed commit message)
  - After a PR is merged, the maintainer (or CI) runs something like entire import --from <fork-url> (or --pr 123)
  - That command fetches entire/checkpoints/v1 from the contributor's fork, finds checkpoints whose IDs match trailers in the upstream history, and merges them into upstream's metadata branch
  - The checkpoint ID in the trailer is the join key — no SHA or tree hash matching needed

### Notes
- tree hash (and commit SHA) aren't needed for cross-repo linking. 
- The trailer is the stable identifier that travels with the commit message through merges.

# Lightweight validation (tests):
- Create a small throwaway repo:
  1. enable Entire
  2. make a few commits with trailers on a fork
  3. merge PRs via each GitHub strategy (merge commit, squash, rebase)
  4. Verify the trailers survive in each case. 

If successful, the import mechanism just needs to git fetch <fork> entire/checkpoints/v1 and match checkpoint IDs.

# Architecture Diagram
Manual-commit strategy: The prepare-commit-msg git hook automatically appends `Entire-Checkpoint: XXXX` to the commit message when the user commits. The user can remove it before saving if they don't want it. (See manual_commit_hooks.go:216)

Auto-commit strategy: The trailer is added programmatically when the strategy creates commits on behalf of the user.                                                                          
                            
The missing piece is the import command on the receiving end — something that fetches a fork's `entire/checkpoints/v1` branch and copies over the matching checkpoint data.


  CONTRIBUTOR'S FORK                              UPSTREAM REPO
  ════════════════                                ════════════════

  git commit
      │
      ▼
  prepare-commit-msg hook
  auto-appends trailer
      │
      ▼
  ┌─────────────────────────────┐                 ┌─────────────────────────────┐
  │ "Add login feature          │                 │ "Add login feature (#42)    │
  │                             │    PR merge     │                             │
  │  Entire-Checkpoint: XXXX"   │ ──────────────► │  Entire-Checkpoint: XXXX"   │
  └─────────────────────────────┘  (trailer       └──────────────┬──────────────┘
                                    survives)                    │
                                                                 │ scan for XXXX
  entire/checkpoints/v1:                                         │
  ┌──────────────────┐                                           ▼
  │ XXXX/            │         entire import               ┌───────────┐
  │ ├── metadata     │ ◄────── --from <fork> ◄──────────── │ found it! │
  │ ├── transcript   │                                     └───────────┘
  │ ├── prompts      │                                           │
  │ └── context      │                                           ▼
  └──────────────────┘
                                                  entire/checkpoints/v1:
                                                  ┌──────────────────┐
                                                  │ XXXX/            │
                                                  │ ├── metadata     │
                                                  │ ├── transcript   │
                                                  │ ├── prompts      │
                                                  │ └── context      │
                                                  └──────────────────┘

The hook runs automatically on every commit when Entire is enabled.
