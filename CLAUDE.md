# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Verve is a distributed AI agent orchestrator platform. It dispatches AI coding agents to work on tasks within user infrastructure. The system has two halves:

1. **Internal Cloud** (we control): API server, Postgres database, and web UI for task management
2. **Orchestrator Worker** (user deploys): Docker container that long-polls for work, runs isolated agents, streams logs, and creates PRs

Key constraint: User source code and secrets never leave their network. We send task descriptions in; we get logs and PR notifications out.

## Important Rules

- **Never build binaries to the project root.** Always use `make build` or output to `bin/` (e.g. `go build -o bin/ ./cmd/...`). The `bin/` directory is git-ignored.

## Commit Convention

This project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/). Goreleaser uses these prefixes to generate changelogs.

**Format:** `type: short description`

| Prefix | Purpose | Changelog |
|--------|---------|-----------|
| `feat:` | New feature | Features |
| `fix:` | Bug fix | Bug fixes |
| `refactor:` | Code restructuring (no behavior change) | Others |
| `docs:` | Documentation only | Excluded |
| `test:` | Adding/updating tests | Excluded |
| `chore:` | Maintenance (deps, configs, scripts) | Excluded |
| `ci:` | CI/CD changes | Excluded |

Examples:
- `feat: add epic planning support`
- `fix: prevent stale tasks from blocking queue`
- `chore: bump Go to 1.25`

## Run Modes

Verve builds a single `verve` CLI binary with three run modes:

- **`verve`** (default): Runs both API server + worker in one process. Best for local development. Auto-enables UI, auto-generates encryption key, uses file-backed SQLite at `~/.local/share/verve/`.
- **`verve api`**: Runs only the API server. Use for distributed deployments or when running the worker separately.
- **`verve worker`**: Runs only the worker. Connects to a remote API server via `--api-url`.

All flags can also be set via environment variables (e.g. `--port` / `PORT`).

## Development Commands

```bash
# Build
make build                        # Build verve binary
make build-agent                  # Build agent Docker image

# UI
make ui-install                   # Install UI dependencies (pnpm)
make ui-dev                       # Start UI dev server
make ui-build                     # Build UI for standalone use
make ui-build-go                  # Build UI into internal/frontend/dist for Go embed

# Generate
make generate                     # Generate sqlc code for postgres and sqlite

# Run
make run                          # Start both API server + worker (combined mode)
make run-api                      # Start API server only
make run-api-pg                   # Start API server with PostgreSQL
make run-worker                   # Start worker only (connects to localhost:7400)

# Test
make test-task                    # Create a test task via curl
make list-tasks                   # List all tasks
make get-task ID=tsk_xxx          # Get specific task details

# Release
make release                      # Tag patch release and publish via goreleaser
make release BUMP=minor           # Tag minor release
make release BUMP=major           # Tag major release

# Clean
make clean                        # Remove binaries and Docker image
make tidy                         # Run go mod tidy
```

## Technology Stack

- **Language**: Go 1.25+
- **CLI Framework**: urfave/cli/v2
- **HTTP Framework**: Echo v4
- **Database**: PostgreSQL (production) / SQLite in-memory (dev)
- **SQL Generation**: sqlc (via `go tool sqlc`)
- **Container Runtime**: Docker (via Docker SDK for Go)
- **Utilities**: `github.com/joshjon/kit` (pgdb, sqlitedb, errtag, id)
- **IDs**: TypeID via `go.jetify.com/typeid`

## Architecture

```
Internal Cloud                          User Environment
┌─────────────────────────┐            ┌─────────────────────────┐
│ Postgres ◄─► API Server │◄── HTTPS ──│ Orchestrator Worker     │
│              ◄─► UI     │            │   └─► Agent containers  │
└─────────────────────────┘            └─────────────────────────┘
```

Local development runs both sides in a single process via `verve` (default mode).

## Package Structure

```
verve/
├── main.go                            # Unified CLI entrypoint (api, worker, combined)
├── internal/
│   ├── app/
│   │   ├── config.go               # Config, PostgresConfig, GitHubConfig
│   │   └── run.go                  # Run (auto-selects postgres or sqlite)
│   ├── keymanager/
│   │   └── keymanager.go           # Encryption key auto-management (~/.config/verve/)
│   ├── task/
│   │   ├── id.go                   # TaskID typed ID (kit/id + typeid)
│   │   ├── task.go                 # Task struct, Status enum, NewTask
│   │   ├── repository.go           # Repository interface
│   │   ├── repository_errors.go    # ErrTagTaskNotFound, ErrTagTaskConflict
│   │   └── store.go                # Store wrapping Repository + pending notification
│   ├── taskapi/
│   │   ├── http_handler.go         # HTTP handlers with Register(group)
│   │   └── http_types.go           # Request/response types
│   ├── github/
│   │   └── client.go               # GitHub API client for PR status checks
│   ├── postgres/
│   │   ├── db.go                   # DB type alias
│   │   ├── gen.go                  # //go:generate sqlc
│   │   ├── task_repo.go            # TaskRepository implements task.Repository
│   │   ├── marshal.go              # sqlc row -> domain entity conversion
│   │   ├── migrations/
│   │   │   ├── fs.go               # //go:embed *.sql
│   │   │   └── 0001_create_tasks.up.sql
│   │   ├── queries/
│   │   │   └── task.sql            # sqlc query definitions
│   │   ├── sqlc.yaml               # sqlc config (engine: postgresql, pgx/v5)
│   │   └── sqlc/                   # generated by sqlc (DO NOT EDIT)
│   ├── sqlite/
│   │   ├── db.go                   # DB type alias
│   │   ├── gen.go                  # //go:generate sqlc
│   │   ├── task_repo.go            # TaskRepository implements task.Repository
│   │   ├── marshal.go              # JSON array handling for SQLite
│   │   ├── migrations/
│   │   │   ├── fs.go               # //go:embed *.sql
│   │   │   └── 0001_create_tasks.up.sql
│   │   ├── queries/
│   │   │   └── task.sql
│   │   ├── sqlc.yaml               # sqlc config (engine: sqlite)
│   │   └── sqlc/                   # generated by sqlc (DO NOT EDIT)
│   └── worker/
│       ├── worker.go               # Polling loop and task execution
│       └── docker.go               # Docker SDK integration
├── agent/
│   ├── Dockerfile                  # Agent container image
│   └── entrypoint.sh               # Agent execution script
├── go.mod
└── Makefile
```

## Database Layer

### Repository Pattern
- Domain types and `Repository` interface defined in `internal/task/`
- PostgreSQL implementation in `internal/postgres/` using pgx/v5
- SQLite implementation in `internal/sqlite/` using database/sql + modernc.org/sqlite
- Both backends are interchangeable via the `task.Repository` interface

### SQLC Conventions
- Query files in `internal/postgres/queries/*.sql` and `internal/sqlite/queries/*.sql`
- Use `-- name: QueryName :one/:many/:exec` comment syntax
- Generated code in `*/sqlc/` directories — never edit manually
- Run `make generate` after changing queries or migrations
- SQLite stores array fields (logs, depends_on) as JSON text

### Migrations
- Embedded via `//go:embed *.sql` in `*/migrations/fs.go`
- Run automatically on server startup via `pgdb.Migrate` / `sqlitedb.Migrate`
- Follow golang-migrate naming: `NNNN_description.up.sql`

## API Structure

Base path: `/api/v1`

```
/tasks
├── POST                     # Create task
├── GET                      # List all tasks
├── /sync                    # POST sync all tasks in review
├── /poll                    # GET long-poll for pending tasks
└── /{task_id}
    ├── GET                  # Get task details with logs
    ├── /logs                # POST logs from worker
    ├── /complete            # POST completion status from worker
    ├── /close               # POST close task with reason
    └── /sync                # POST sync single task PR status
```

## Worker-Cloud Communication

Worker communicates with API server via REST/JSON:
- `GET /tasks/poll`: Long-poll to claim pending tasks
- `POST /tasks/{id}/logs`: Send collected agent logs
- `POST /tasks/{id}/complete`: Report success/failure

## Entity Model

### Task Status Lifecycle
```
pending → running → review → merged
                  → closed
                  → failed
```

### Entity Identity Pattern
Use TypeID prefixes for entity IDs:
- `tsk_*` = Task (e.g., `tsk_01HQXYZ...`)

IDs use `github.com/joshjon/kit/id` with `go.jetify.com/typeid` for type-safe prefixed UUIDs.

## Key Patterns

### Error Handling
Use semantic error types via `github.com/joshjon/kit/errtag`:
- `errtag.NotFound` → 404
- `errtag.Conflict` → 409
- `errtag.InvalidArgument` → 400

Database implementations tag errors in `tagTaskErr()`:
- `pgx.ErrNoRows` / `sql.ErrNoRows` → `ErrTagTaskNotFound`
- Unique violation → `ErrTagTaskConflict`

### Store Pattern
`task.Store` wraps `task.Repository` adding:
- Dependency validation on create
- Pending task notification channel for long-poll

### Log Streaming
Worker streams logs from Docker container in real-time:
1. Attaches to container stdout/stderr with `Follow=true`
2. Demultiplexes the Docker stream using `stdcopy`
3. Buffers lines and sends batches to API server every 2 seconds (or when buffer reaches 50 lines)
4. UI can poll `/tasks/{id}` to see logs incrementally as the agent runs

### Agent Isolation
Uses Docker-in-Docker approach:
- Each task spawns an isolated Docker container
- Container receives task via environment variables (TASK_ID, TASK_DESCRIPTION)
- Container is automatically removed after execution
- Agent image: `verve-agent:latest`

## Important Notes

- User code never leaves their network - only task descriptions flow in, logs and PR notifications flow out
- Workers authenticate with per-user API keys
- Task queue uses long-polling (worker initiates connection, server holds until task available)
- Agents are ephemeral - one process per task, destroyed after completion
- PR creation happens on user side using their Git credentials
