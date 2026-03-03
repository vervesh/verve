# Design

## Architecture

The system is split into two halves:

- **Internal cloud** (API server, database, web UI) — manages tasks, stores logs, serves the dashboard
- **Worker** (user deploys) — long-polls for tasks, spawns agent containers, streams logs, creates PRs

User source code and secrets never leave their network. Task descriptions flow in, logs and PR notifications flow out.

```
Internal Cloud                          User Environment
┌───────────────────────────┐          ┌───────────────────────────┐
│ Postgres ◄─► API Server   │◄─ HTTPS ─│ Worker                    │
│              ◄─► Web UI   │          │   └─► Agent containers    │
└───────────────────────────┘          └───────────────────────────┘
```

## Task Lifecycle

```
pending → running → review → merged
                           → closed
                  → failed
```

1. User creates a task in the UI with a description and target repository
2. Task enters the queue as `pending`
3. Worker claims it via long-poll — status becomes `running`
4. Agent works inside an isolated Docker container; logs stream back in real-time
5. Agent pushes a branch and opens a PR — status becomes `review`
6. PR status is monitored (CI, merge state). CI failures trigger automatic retries
7. Once merged, status becomes `merged`

## Worker

The worker is a Go binary distributed as a Docker container. It runs a polling loop:

1. Long-poll `GET /tasks/poll` to claim the next pending task
2. Spawn an ephemeral Docker container with the agent image
3. Stream container logs to the API server in batches (every 2s or 50 lines)
4. Parse structured markers from agent output (`VERVE_PR_CREATED`, `VERVE_STATUS`, `VERVE_COST`)
5. Report task completion, clean up the container, loop

Workers receive GitHub tokens and repo details from the API server per-task — no local credential configuration needed.

## Agent

Each task runs in an isolated Docker container running Claude Code. The agent:

1. Clones the repository
2. Reads the task description and acceptance criteria
3. Makes code changes, runs tests
4. Commits to a `verve/task-{id}` branch and pushes
5. Opens a PR via the GitHub API

The base image (`node:22-alpine` + Claude Code) can be extended with custom Dockerfiles for project-specific dependencies.

## Database

- **PostgreSQL** in production, **SQLite** (in-memory) for development
- Repository pattern with interchangeable implementations
- SQL queries defined in `internal/{postgres,sqlite}/queries/`, generated via sqlc
- Embedded migrations run automatically on startup

## API

Base path: `/api/v1`

- `POST /tasks` — create task
- `GET /tasks` — list tasks
- `GET /tasks/{id}` — get task with logs
- `GET /tasks/poll` — long-poll for pending tasks (worker)
- `POST /tasks/{id}/logs` — append logs (worker)
- `POST /tasks/{id}/complete` — report completion (worker)
- `POST /tasks/{id}/close` — close task
- `POST /tasks/{id}/sync` — sync PR status from GitHub
- `GET /events` — SSE stream for real-time UI updates
