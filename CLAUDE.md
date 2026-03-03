# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Verve is a distributed AI agent orchestrator platform. It dispatches AI coding agents to work on tasks within user infrastructure. The system has two halves:

1. **Internal Cloud** (we control): API server, Postgres database, and web UI for task management
2. **Orchestrator Worker** (user deploys): Docker container that long-polls for work, runs isolated agents, streams logs, and creates PRs

Key constraint: User source code and secrets never leave their network. We send task descriptions in; we get logs and PR notifications out.

## Important Rules

- **Never build binaries to the project root.** The root directory is not gitignored, so any binary there will pollute git status. **Never run `go build .` or `go build` without `-o`** — this outputs a binary to the current directory. Always use `go build -o bin/ .` or `make build`. The `bin/` directory is git-ignored.
- **Always check if UI changes are required when updating backend APIs.** When adding, modifying, or removing API endpoints, request/response types, or entity fields, check if the UI needs corresponding updates. This includes: TypeScript type definitions in `ui/src/lib/models/`, API client methods in `ui/src/lib/api-client.ts`, and Svelte components in `ui/src/lib/components/` that render or interact with the changed data. Failing to update the UI alongside backend changes results in an incomplete feature.

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
# Build & Run
go build -o bin/verve .           # Build verve binary
./bin/verve                       # Start both API server + worker (combined mode)
./bin/verve api                   # Start API server only
./bin/verve worker                # Start worker only

# Agent Images
make build-agent                  # Build base agent Docker image (verve:base)
make build-agent-dev              # Build dev agent image (verve:dev)
make push-agent                   # Push verve:base to ghcr.io
make push-agent TAG=base-0.2.0   # Push versioned tag

# UI
make ui-install                   # Install UI dependencies (pnpm)
make ui-dev                       # Start UI dev server
make ui-build                     # Build UI for standalone use
make ui-build-go                  # Build UI into internal/frontend/dist for Go embed

# Code Generation
make generate                     # Generate sqlc code for postgres and sqlite

# Docker Compose
make up                           # Build agent + start compose stack
make up-build                     # Rebuild all containers and start
make down                         # Stop compose stack
make logs                         # Tail compose logs

# Release & Deploy
make release                      # Tag patch release and publish via goreleaser
make release BUMP=minor           # Tag minor release
make release BUMP=major           # Tag major release
make deploy                       # Deploy to Fly.io

# Cleanup
make clean                        # Remove binaries, UI dist, and agent image
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
│   ├── logkey/
│   │   └── keys.go                 # Structured request log keys (TaskID, RepoID, EpicID)
│   ├── keymanager/
│   │   └── keymanager.go           # Encryption key auto-management (~/.config/verve/)
│   ├── metric/
│   │   └── metric.go               # Metrics types and Compute function
│   ├── task/
│   │   ├── id.go                   # TaskID typed ID (kit/id + typeid)
│   │   ├── task.go                 # Task struct, Status enum, NewTask
│   │   ├── repository.go           # Repository interface
│   │   ├── repository_errors.go    # ErrTagTaskNotFound, ErrTagTaskConflict
│   │   └── store.go                # Store wrapping Repository + pending notification
│   ├── taskapi/
│   │   ├── http_handler.go         # Task HTTP handlers (CRUD, lifecycle, sync)
│   │   └── http_types.go           # Task request/response types
│   ├── repoapi/
│   │   ├── http_handler.go         # Repo CRUD HTTP handlers
│   │   └── http_types.go           # Repo request types
│   ├── metricapi/
│   │   └── http_handler.go         # GET /metrics endpoint
│   ├── settingapi/
│   │   ├── http_handler.go         # Settings HTTP handlers (GitHub token, default model)
│   │   └── http_types.go           # Settings request/response types
│   ├── eventapi/
│   │   └── http_handler.go         # SSE /events endpoint
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
├── docs/
│   ├── FEATURES.md                 # Feature documentation
│   └── DESIGN.md                   # Architecture and design decisions
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

Base path: `/api/v1`. Handlers are split by concern:

```
repoapi:    /repos, /repos/:repo_id, /repos/available
taskapi:    /repos/:repo_id/tasks, /tasks/:id, /tasks/:id/{action}
settingapi: /settings/github-token, /settings/default-model, /settings/models
metricapi:  /metrics
eventapi:   /events (SSE)
epicapi:    /repos/:repo_id/epics, /epics/:id, /epics/:id/{action}
agentapi:   /agent/tasks/poll, /agent/tasks/:id/{logs,complete,heartbeat}
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

### Handler Conventions

HTTP handlers follow `kit/server` conventions. Middleware handles error-to-HTTP mapping automatically.

**Request binding**: Use `server.BindRequest[T](c)` where `T` implements `Validate() error` (valgo validation).

**Request types**: Defined in `http_types.go` per package. Path params use `param:"id"` tags, body fields use `json:` tags. Each type has a `Validate()` method using valgo.

**Responses**:
- Single entity: `server.SetResponse(c, code, entity)` → `{"data": ...}`
- List: `server.SetResponseList(c, code, items, "")` → `{"data": [...]}`
- No body: `c.NoContent(http.StatusNoContent)` (for deletes, actions)

**Error handling**: Return errors directly — middleware maps them:
- valgo validation errors → 400
- `errtag.NotFound` → 404
- `errtag.Conflict` → 409
- `errtag.InvalidArgument` → 400
- `echo.NewHTTPError(code, msg)` → custom HTTP code (e.g. 503)
- Error response format: `{"error": {"message": "...", "details": [...]}}`

**Log context**: Set entity IDs via `c.Set(logkey.TaskID, id.String())` for structured request logging.

**ID validation pattern**:
```go
func (r MyRequest) Validate() error {
    return valgo.In("params", valgo.Is(task.TaskIDValidator(r.ID, "id"))).ToError()
}
```
Then in handler: `id := task.MustParseTaskID(req.ID)` (safe after validation).

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
- Agent image: `verve:base` (stack variants: `verve:golang`, `verve:python`, etc.)

## Testing

### Prefer SQLite over mocks
When writing tests that need a repository or store, **always use a real in-memory SQLite-backed repository** instead of hand-written mocks — provided a SQLite implementation exists for that repository in `internal/sqlite/`. Only fall back to mocks when no SQLite implementation is available (e.g. external service clients like GitHub).

Use `sqlite.NewTestDB(t)` to create an in-memory database with all migrations applied and automatic cleanup:

```go
db := sqlite.NewTestDB(t)
taskRepo := sqlite.NewTaskRepository(db)
repoRepo := sqlite.NewRepoRepository(db)
settingRepo := sqlite.NewSettingRepository(db)
```

Available SQLite repositories: `TaskRepository`, `RepoRepository`, `EpicRepository`, `SettingRepository`, `GitHubTokenRepository`.

To seed test data, use repo methods directly (e.g. `taskRepo.CreateTask`, `taskRepo.UpdateTaskStatus`, `taskRepo.SetTaskPullRequest`). To verify state after handler calls, re-read from the database rather than checking in-memory structs.

### HTTP handler tests with testutil

HTTP handler tests use `kit/testutil` and `kit/server` to spin up a real server and make typed HTTP requests. Each handler package has a `http_handler_fixture_test.go` with a test fixture and a `http_handler_test.go` with tests. Tests use `_test` package suffix (e.g. `package taskapi_test`).

**Fixture pattern** — creates a real `server.Server`, registers the handler, and provides URL helpers + seed helpers:

```go
func newFixture(t *testing.T) *fixture {
    db := sqlite.NewTestDB(t)
    // ... create repos, stores, handler ...
    srv, err := server.NewServer(testutil.GetFreePort(t))
    srv.Register("/api/v1", handler)
    go srv.Start()
    srv.WaitHealthy(10, 100*time.Millisecond)
    t.Cleanup(func() { srv.Stop(context.Background()) })
    return &fixture{Server: srv, ...}
}
```

**Success tests** — use `testutil.Get/Post/Put/Delete` with typed request structs and `server.Response[T]` / `server.ResponseList[T]` envelopes:

```go
req := taskapi.CreateTaskRequest{Title: "Fix bug", Description: "desc"}
res := testutil.Post[server.Response[task.Task]](t, f.repoTasksURL(), req)
assert.Equal(t, "Fix bug", res.Data.Title)
```

**Error tests** — use `testutil.DefaultClient` directly and assert on `StatusCode`:

```go
httpRes := doJSON(t, http.MethodPost, url, req)
defer httpRes.Body.Close()
assert.Equal(t, http.StatusBadRequest, httpRes.StatusCode)
```

**POST → 204 NoContent** (e.g. AppendLogs, CompleteTask) — use a `postNoContent` helper since `testutil.Post[R]` tries to decode the body:

```go
postNoContent(t, f.taskActionURL(tsk.ID, "complete"), req)
```

**DELETE with JSON body** (e.g. RemoveDependency) — use a `doJSON` helper since `testutil.Delete` doesn't support request bodies.

### Table-driven tests

Use table-driven tests when a function/endpoint has **multiple input variations that share the same assertion logic** (e.g. validation tests, parsing tests, status transitions). Keep each subtest self-contained and the test table close to the `for` loop.

```go
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"valid input", "epc_01HQXYZ...", false},
    {"empty string", "", true},
    {"wrong prefix", "tsk_01HQXYZ...", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        _, err := ParseEpicID(tt.input)
        if tt.wantErr {
            assert.Error(t, err)
        } else {
            assert.NoError(t, err)
        }
    })
}
```

**When to use table-driven**: ID parsing/validation, request validation (multiple invalid fields), status transition checks, any case with ≥3 similar subtests.

**When NOT to use**: Integration tests with complex setup/teardown, tests where each case has unique assertions, or tests with ≤2 trivially different cases.

## Important Notes

- User code never leaves their network - only task descriptions flow in, logs and PR notifications flow out
- Workers authenticate with per-user API keys
- Task queue uses long-polling (worker initiates connection, server holds until task available)
- Agents are ephemeral - one process per task, destroyed after completion
- PR creation happens on user side using their Git credentials
