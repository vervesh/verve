# Tome — Agent Session Memory

Tome is a standalone CLI for recording and searching agent session history. It gives agents institutional memory — the ability to learn from previous sessions, avoid repeating mistakes, and build on past work.

## Why

Verve's agent containers are ephemeral. Each task spawns a fresh Docker container with no knowledge of what previous agents discovered. Without session memory, agents rediscover the same gotchas, repeat failed approaches, and miss patterns that earlier agents already found.

Tome solves this by giving agents an on-demand search tool — like `grep` but for institutional knowledge rather than code. Agents query for relevant sessions as they work, and record what they learned when they finish.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ Agent Container                                     │
│                                                     │
│  Agent works on task...                             │
│    ├─ tome search "auth middleware"    ← on-demand  │
│    ├─ tome search --file "src/api/"   ← filtered   │
│    └─ tome record --summary "..."     ← on finish  │
│                                                     │
│  /cache/tome/data.db  ← mounted from host volume   │
└─────────────────────────────────────────────────────┘
        │ (Docker cache volume)
┌───────┴──────────────────┐
│ Host: ~/.cache/verve/tome│ ← persists across containers
└──────────────────────────┘

Standalone user:
  $ cd my-repo
  $ tome search "authentication"   ← reads .tome/data.db
  $ tome record --summary "..."    ← writes .tome/data.db
```

Tome has **no dependency on Verve**. It's a general-purpose tool that works for both Verve agents and standalone Claude Code users.

### Data directory resolution

1. `TOME_DIR` env var — used in Docker containers (`/cache/tome`)
2. `.tome/` in git repo root — for standalone users

### Database

SQLite via `modernc.org/sqlite` (pure Go, no CGO). The database file is `data.db` inside the data directory. Schema auto-migrates on first use.

## Search

Tome uses a hybrid search system that combines keyword matching with semantic similarity.

### BM25 (keyword search)

FTS5 full-text search across session summaries, learnings, and tags. Fast, precise, and works from the first session. This is the baseline — it always works.

### LSA (semantic search)

Latent Semantic Analysis captures term co-occurrence patterns to find semantically related sessions. For example, a search for "authentication" will also surface sessions about "login flow" or "OAuth tokens" even if they don't contain the exact word "authentication".

**How it works:**
1. Tokenize all sessions (lowercase, stop word removal)
2. Build a TF-IDF weighted document-term matrix
3. SVD decomposition reduces it to ~128 dimensions (concept space)
4. Query is projected into the same space
5. Sessions are ranked by cosine similarity to the query

**Requirements:** Needs at least 2 sessions to build an index. Terms must appear in at least 2 sessions to be included in the vocabulary.

### Hybrid scoring

When LSA is available, both scores are combined:

```
final_score = 0.4 × normalize(bm25) + 0.6 × normalize(lsa)
```

LSA gets higher weight because it captures relationships that keyword search misses. If LSA is unavailable (< 2 sessions, build failure), search gracefully degrades to BM25-only.

## Git Sync

Sessions can be synchronized across machines via git orphan branches. Each user or worker pushes sessions to their own branch, and pulls from all branches on the remote.

### Branch layout

```
Git Remote
├── main                                # Normal code
├── tome/context/alice@example.com      # Alice's sessions
├── tome/context/bob@example.com        # Bob's sessions
└── tome/context/shared                 # Shared/worker sessions
```

### Wire format

Sessions are stored as JSONL (one JSON object per line) in a file called `sessions.jsonl` on each branch:

```jsonl
{"id":"abc-123","summary":"Added JWT refresh tokens","learnings":"Redis required for blacklist","tags":["auth","jwt"],"files":["src/auth.go"],"status":"succeeded","author":"alice@example.com","created_at":"2026-03-10T14:30:00Z"}
```

### How sync works

**Push:** Queries the database for unexported sessions, appends them to `sessions.jsonl`, commits using git plumbing commands (`hash-object`, `mktree`, `commit-tree`) without polluting the working tree, and pushes to the remote. Sessions are then marked as exported.

**Pull:** Fetches all `tome/context*` branches from the remote, reads each branch's `sessions.jsonl`, and imports sessions into the local database. Session IDs are used for deduplication — re-pulling is safe and idempotent.

**Conflict avoidance:** Each user pushes only to their own branch, so there are no write conflicts. All branches are merged into the local database on pull.

## Verve integration

### Docker image

The `tome` binary is cross-compiled for Linux and included in the agent Docker image at `/usr/local/bin/tome`. The `TOME_DIR` env var is set to `/cache/tome`, which maps to the host's cache volume so sessions persist across containers.

### Worker

When cache is enabled, the worker sets `TOME_DIR=/cache/tome` in the container environment. The cache volume (`~/.cache/verve` → `/cache`) is already mounted, so `/cache/tome/` is automatically available.

### Agent prompt

When `tome` is available in the container, the agent receives these instructions:

```
SESSION MEMORY: You have access to `tome` for searching and recording session history.
- Before starting, search for relevant past sessions: `tome search "relevant topic"`
- Filter by files touched: `tome search --file "src/auth/" "query"`
- After completing work, record what you learned: `tome record --summary "What you did" --learnings "Key findings and gotchas" --tags "comma,separated" --files "files,touched" --status succeeded`
- View recent sessions: `tome log`
```

## Package structure

```
cmd/tome/
    main.go                     # CLI entry point (urfave/cli/v2)

internal/tome/
    tome.go                     # Tome struct: Open, Close, Log, LSA management
    session.go                  # Session, SearchOpts, SearchResult types
    record.go                   # Record() — insert session
    search.go                   # Search() — hybrid BM25+LSA
    lsa.go                      # TF-IDF matrix, SVD, cosine similarity
    tokenizer.go                # Text tokenization and stop words
    sync.go                     # Git orphan branch sync (pull/push)
    jsonl.go                    # JSONL encode/decode for wire format
    format.go                   # Text and JSON output formatting
    tome_test.go                # Core integration tests
    lsa_test.go                 # LSA and hybrid search tests
    sync_test.go                # Git sync tests
    migrations/
        fs.go                   # //go:embed *.sql
        0001_init.up.sql        # sessions table + FTS5
        0002_sync_metadata.up.sql  # exported flag + author column
```

## CLI reference

```
tome search <query>              # Hybrid BM25+LSA search
tome search --bm25-only <query>  # Keyword-only search
tome search --file "path" <q>    # Filter by files touched
tome search --status failed <q>  # Filter by outcome
tome search --limit 3 <query>    # Top N results (default: 5)
tome search --json <query>       # JSON output

tome record \
  --summary "What you did" \
  --learnings "Key findings" \
  --tags "auth,jwt" \
  --files "src/auth.go" \
  --status succeeded \
  --author "user@example.com"    # Auto-detected from git config

tome log                         # Recent sessions (default: 10)
tome log --limit 5 --json        # Last 5, JSON format

tome index                       # Rebuild LSA index (diagnostics)
tome init                        # Initialize database explicitly

tome sync                        # Pull + push (default)
tome sync --pull                 # Import from remote only
tome sync --push                 # Export to remote only
tome sync --branch "custom"      # Override branch name
```

---

## Testing guide

This section walks through testing the full tome feature set end-to-end. All commands run from the repo root.

### Prerequisites

```bash
# Build the tome binary
make build-tome

# Verify it runs
./bin/tome --help
```

### 1. Initialize and record sessions

```bash
# Initialize (optional — auto-inits on first record)
./bin/tome init

# Record a few sessions with different topics
./bin/tome record \
  --summary "Added JWT authentication middleware" \
  --learnings "Token validation uses Bearer scheme. Refresh tokens stored in httponly cookies. Redis required for token blacklist." \
  --tags "auth,jwt,middleware" \
  --files "src/auth/middleware.go,src/auth/tokens.go" \
  --status succeeded

./bin/tome record \
  --summary "Implemented user login flow" \
  --learnings "Password hashing with bcrypt. Session cookies for login state. Auth redirect on expired session." \
  --tags "auth,login,user" \
  --files "src/auth/login.go,src/user/handler.go" \
  --status succeeded

./bin/tome record \
  --summary "Fixed rate limiter for API endpoints" \
  --learnings "Sliding window algorithm for rate limiting. Middleware chain executes rate check before handler. Tests use a mock clock." \
  --tags "api,rate-limiting,middleware" \
  --files "src/api/ratelimit.go,src/api/middleware.go" \
  --status succeeded

./bin/tome record \
  --summary "Database migration for user accounts" \
  --learnings "Schema migration adds email verification column. Foreign key constraints for user sessions table." \
  --tags "database,migration" \
  --files "migrations/003_user_accounts.sql" \
  --status succeeded

./bin/tome record \
  --summary "Failed to add password reset emails" \
  --learnings "SMTP config was missing in test environment. Reset token expiry logic was wrong — used seconds instead of hours." \
  --tags "user,email" \
  --files "src/user/reset.go,src/email/templates.go" \
  --status failed

# Verify they're stored
./bin/tome log
```

**Expected:** 5 sessions listed, most recent first, with summaries, tags, files, and relative timestamps.

### 2. BM25 keyword search

```bash
# Basic keyword search
./bin/tome search "authentication"

# Should find the JWT and login sessions

# Filter by status
./bin/tome search --status failed "email"

# Should find only the failed password reset session

# Filter by file path
./bin/tome search --file "src/api/" "middleware"

# Should find the rate limiter session (file matches src/api/)

# Limit results
./bin/tome search --limit 1 "auth"

# Force BM25-only mode
./bin/tome search --bm25-only "middleware"

# JSON output
./bin/tome search --json "auth"
```

### 3. Hybrid search (LSA semantic matching)

With 5 sessions recorded, LSA is active. Test semantic discovery:

```bash
# Search for "login" — should find JWT/auth sessions too via semantic similarity
./bin/tome search "login"

# Search for "security" — should surface auth-related sessions
# even though none contain the word "security"
./bin/tome search "security"

# Compare hybrid vs BM25-only to see the difference
./bin/tome search "token"
./bin/tome search --bm25-only "token"

# The hybrid search should return at least as many results
```

### 4. LSA index management

```bash
# Manually rebuild and inspect the index
./bin/tome index

# Expected output like: Built LSA index: 5 sessions, N terms, K dimensions
# Dimensions will be 4 (min of 128, numDocs-1)
```

### 5. Git sync

This requires a git remote. Use a temporary bare repo to test locally:

```bash
# Set up a test remote
TMPDIR=$(mktemp -d)
git init --bare --initial-branch=main "$TMPDIR/remote.git"

# Create two "clones" simulating two users
git clone "$TMPDIR/remote.git" "$TMPDIR/clone1"
git clone "$TMPDIR/remote.git" "$TMPDIR/clone2"

# Configure identities
git -C "$TMPDIR/clone1" config user.email "alice@example.com"
git -C "$TMPDIR/clone1" config user.name "Alice"
git -C "$TMPDIR/clone2" config user.email "bob@example.com"
git -C "$TMPDIR/clone2" config user.name "Bob"

# Create an initial commit so the remote isn't empty
echo "# test" > "$TMPDIR/clone1/README.md"
git -C "$TMPDIR/clone1" add README.md
git -C "$TMPDIR/clone1" commit -m "init"
git -C "$TMPDIR/clone1" push -u origin main
git -C "$TMPDIR/clone2" pull
```

Now test sync from clone1 (Alice):

```bash
# Record a session in clone1
cd "$TMPDIR/clone1"
TOME_DIR="$TMPDIR/tome1" tome record \
  --summary "Alice added auth middleware" \
  --learnings "Bearer token validation in middleware" \
  --tags "auth" \
  --author "alice@example.com"

# Push to remote
TOME_DIR="$TMPDIR/tome1" tome sync --push --author "alice@example.com"

# Expected: "Exported 1 sessions."

# Verify the orphan branch exists
git branch -a | grep tome

# Expected: tome/context/alice@example.com
```

Pull from clone2 (Bob):

```bash
cd "$TMPDIR/clone2"
TOME_DIR="$TMPDIR/tome2" tome sync --pull

# Expected: "Imported 1 sessions."

# Verify Alice's session is searchable
TOME_DIR="$TMPDIR/tome2" tome search "auth"

# Expected: Alice's session appears with Author: alice@example.com
```

Bidirectional sync:

```bash
# Bob records a session
TOME_DIR="$TMPDIR/tome2" tome record \
  --summary "Bob fixed rate limiter" \
  --learnings "Sliding window was off by one" \
  --tags "api" \
  --author "bob@example.com"

# Bob pushes
cd "$TMPDIR/clone2"
TOME_DIR="$TMPDIR/tome2" tome sync --push --author "bob@example.com"

# Alice pulls — should get Bob's session
cd "$TMPDIR/clone1"
TOME_DIR="$TMPDIR/tome1" tome sync --pull

# Expected: "Imported 1 sessions."
TOME_DIR="$TMPDIR/tome1" tome log

# Expected: Both sessions listed

# Pull again — should be idempotent
TOME_DIR="$TMPDIR/tome1" tome sync --pull

# Expected: "Already up to date."
```

Push idempotency:

```bash
# Push again with no new sessions
cd "$TMPDIR/clone1"
TOME_DIR="$TMPDIR/tome1" tome sync --push --author "alice@example.com"

# Expected: "Already up to date." (nothing to export)
```

Verify JSONL on the branch:

```bash
cd "$TMPDIR/clone1"
git show tome/context/alice@example.com:sessions.jsonl

# Expected: One-line JSON per session with RFC3339 timestamps
```

Clean up:

```bash
rm -rf "$TMPDIR"
```

### 6. Run the automated test suite

```bash
# All tome tests (core + LSA + sync)
go test ./internal/tome/... -v -count=1

# Expected: 25 tests pass
#   9 core tests (TestRecordAndSearch, TestLog, etc.)
#   9 LSA tests (TestBuildLSAIndex, TestHybridSearch*, etc.)
#   7 sync tests (TestSyncPush*, TestSyncPull*, TestSyncBidirectional, etc.)
```

### 7. Docker integration (requires Docker)

```bash
# Build agent image with tome included
make build-agent

# Verify tome is in the image
docker run --rm verve:base tome --help

# Verify TOME_DIR is set
docker run --rm verve:base env | grep TOME_DIR

# Expected: TOME_DIR=/cache/tome
```
