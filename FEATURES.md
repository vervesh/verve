# Features

## Unified CLI Binary

- **Single binary**: One `verve` binary serves as both API server and worker
- **Combined mode** (`verve`): Runs API + worker together for single-command local development
- **API-only mode** (`verve api`): Runs just the API server for distributed deployments
- **Worker-only mode** (`verve worker`): Runs just the worker, connects to a remote API server
- **Auto-managed encryption key**: Generates and stores encryption key at `~/.config/verve/config.json` in combined mode
- **Persistent SQLite**: Combined mode defaults to file-backed SQLite at `~/.local/share/verve/`
- **Flag/env parity**: Every flag has an env var equivalent (e.g. `--port` / `PORT`)

## Task Management

- **Six-state lifecycle**: `pending` → `running` → `review` → `merged` / `closed` / `failed`
- **TypeID identifiers**: Tasks use prefixed UUIDs (`tsk_*`) for type-safe identity
- **Task dependencies**: Tasks can depend on other tasks, with validation and execution gating
- **Acceptance criteria**: Optional criteria passed to the agent for validation and reporting
- **Optimistic locking**: Concurrent task claiming without race conditions

## Retry System

- **Configurable retries**: Up to 5 attempts per task (default)
- **Categorized failures**: Retry reasons tracked by category (`ci_failure`, `merge_conflict`)
- **Retry context**: CI failure logs (up to 4KB) and previous agent status preserved across retries
- **Circuit breaker**: Fast-fails after 2 consecutive same-category failures to prevent infinite loops
- **Budget enforcement**: Tasks fail automatically if cumulative cost exceeds `max_cost_usd`

## Cost Tracking

- **Per-task cost accumulation**: Costs reported by the agent via `VERVE_COST` marker
- **Budget limits**: Optional `max_cost_usd` per task with automatic enforcement on retry
- **UI display**: Current cost and budget shown on task detail page and task cards

## Agent Execution

- **Docker isolation**: Each task runs in an ephemeral container, automatically cleaned up
- **Claude Code integration**: Stream-JSON output mode with model selection (haiku, sonnet, opus); supports both API key (`ANTHROPIC_API_KEY`) and OAuth token (`CLAUDE_CODE_OAUTH_TOKEN`) for subscription-based auth
- **Per-task model selection**: Choose Claude model (haiku, sonnet, opus) per task at creation time; falls back to server-wide default model setting, then sonnet
- **Branch management**: Auto-creates `verve/task-{id}` branches; reuses on retry with rebase
- **PR creation**: Automatic PR with Claude-generated title/description via GitHub API
- **Dry run mode**: Skip Claude API calls for testing; creates dummy changes with dry-run label
- **Structured agent status**: JSON output with `files_modified`, `tests_status`, `confidence`, `blockers`, `criteria_met`, `notes`

## Prerequisite Checks

- **Multi-language detection**: Go, Python, Rust, Java/Kotlin (Gradle/Maven), Ruby, PHP, .NET, Swift
- **File-based detection**: Scans for manifest files (`go.mod`, `requirements.txt`, `Cargo.toml`, etc.)
- **Description-based detection**: Keyword matching in task descriptions for empty repos
- **Structured failure reporting**: Missing tools reported with installation instructions
- **Dockerfile generation**: Claude generates a suggested Dockerfile tailored to the project when prerequisites are missing, displayed in the UI with a copy button
- **No wasted tokens**: Checks run before Claude, so API costs are not incurred on prerequisite failures (only the Dockerfile generation call is made)

## WIP Commit Preservation

- **Exit trap**: Agent pushes uncommitted work to the branch on failure via `push_wip()`
- **Periodic WIP commits**: Agent prompted to make intermediate commits for recovery
- **Previous progress context**: Retried agents receive git log of previous commits to avoid redoing work

## Heartbeat & Stale Task Recovery

- **Worker heartbeats**: Workers send `POST /tasks/:id/heartbeat` every 30 seconds during execution
- **Background reaper**: Server detects running tasks with no heartbeat and marks them as failed
- **Configurable timeout**: `TASK_TIMEOUT` env var (default: 5 minutes) controls stale detection threshold

## Epics

- **AI-powered task planning**: Create an epic with a title and description; an AI agent analyzes the codebase and generates a task breakdown
- **Worker-side planning**: Epic planning runs on worker infrastructure (user's network) — same as task execution, so code never leaves the user's environment
- **Iterative feedback loop**: Send feedback to the planning agent to refine the task breakdown; agent re-plans in real-time
- **Proposed task editing**: Edit, add, or remove proposed tasks before confirming
- **Task dependencies**: Proposed tasks include `depends_on` relationships, preserved when creating real tasks
- **Acceptance criteria**: Each proposed task includes testable acceptance criteria
- **Epic confirmation**: Confirm an epic to create all proposed tasks at once, with optional "hold" mode
- **Separate epics dashboard**: Dedicated epics view accessible via sidebar navigation
- **Planning status indicators**: UI shows "Waiting for worker..." (unclaimed) vs "Agent is planning..." (claimed and active)
- **Session log**: Real-time planning session log showing system messages and user feedback
- **Idle timeout**: Agent containers released after 15 minutes of inactivity
- **Priority scheduling**: Epics are claimed before tasks in the unified work queue

## Worker

- **Unified work queue**: Single poll endpoint (`/api/v1/agent/poll`) claims both epics and tasks, with epics having higher priority
- **Long-poll task claiming**: Atomic status transitions prevent duplicate claims
- **Server-managed credentials**: Workers receive GitHub token and repo info from the API server per-task — no local token or repo configuration needed
- **HTTPS transport security**: Tokens sent over HTTPS (TLS); worker warns on startup if API URL is plain HTTP
- **Configurable concurrency**: `MAX_CONCURRENT_TASKS` with semaphore-based control (default: 3)
- **Sequential mode**: Single-task execution for network-restricted environments
- **Graceful shutdown**: Waits for active tasks to complete before stopping
- **Marker protocol**: Parses structured markers from agent output (`VERVE_PR_CREATED`, `VERVE_STATUS`, `VERVE_COST`, `VERVE_PREREQ_FAILED`)
- **Epic planning support**: Workers run long-lived agent containers for epic planning with heartbeats and feedback polling

## Log Streaming

- **Real-time batching**: Logs sent every 2 seconds or when buffer reaches 50 lines
- **Docker demultiplexing**: stdout/stderr separated via `stdcopy`
- **SSE streaming**: Dedicated `/tasks/{id}/logs` endpoint with historical replay
- **Per-attempt logs**: Logs tagged with attempt number; retries preserve previous attempt logs
- **Tabbed log viewer**: UI shows attempt tabs when task has multiple attempts, with auto-switch to latest
- **Auto-scroll UI**: Log viewer with auto-scroll that disables on manual scroll

## GitHub Integration

- **Repository management**: Add/remove repos, list accessible repos for authenticated user
- **PR status sync**: Checks merged status, CI results, and mergeability
- **CI failure analysis**: Fetches failed check run logs (last 150 lines of the failed step, 8KB total)
- **Background sync**: Every 30 seconds, syncs all tasks in `review` status
- **Auto-retry on CI failure**: Retries with `ci_failure` category and truncated logs as context
- **Auto-retry on merge conflict**: Retries with `merge_conflict` category for automatic rebase

## Multi-Repository Support

- **Repo-scoped tasks**: Each task belongs to a specific repository
- **Repo selector UI**: Dashboard filters by selected repository
- **Server-provided repo info**: Workers receive repo details from the server when claiming tasks
- **Repo-filtered events**: SSE subscriptions scoped to selected repository

## API

- **RESTful endpoints**: Full CRUD for tasks, epics, and repos under `/api/v1`
- **Agent API**: Dedicated `/api/v1/agent/` endpoints for worker/agent communication
- **Unified poll**: `GET /agent/poll` claims epics (priority) or tasks with 30-second long-poll
- **SSE events**: `GET /events` streams `task_created`, `task_updated`, `logs_appended`
- **Task operations**: Create, list, get, close, complete, sync, append logs, retry, feedback
- **Epic operations**: Create, list, get, confirm, close, propose tasks, poll feedback, send messages
- **Repo operations**: List, add, remove, list available from GitHub

## Database

- **Dual backend**: PostgreSQL (production) and SQLite in-memory (development)
- **Repository pattern**: Interface-based abstraction with interchangeable implementations
- **Auto-migrations**: Embedded SQL migrations run on startup
- **sqlc generation**: Type-safe queries generated from SQL definitions
- **PostgreSQL features**: Connection pooling (pgx/v5), NOTIFY/LISTEN for cross-instance events, ENUM types, array support
- **SQLite features**: Zero-config in-memory mode, JSON array encoding for complex fields

## Event System

- **In-process fan-out**: Broker distributes events to SSE subscribers with buffered channels
- **PostgreSQL NOTIFY/LISTEN**: Multi-instance event distribution with auto-reconnect
- **Event types**: `task_created`, `task_updated`, `logs_appended`
- **Init snapshot**: SSE connections receive full task list on connect

## UI

- **Sidebar navigation**: Tasks and Epics separated into dedicated dashboards via sidebar menu
- **Kanban dashboard**: Six status columns with task count badges
- **Epics dashboard**: Grid of epic cards with status filtering, separate from tasks
- **Real-time updates**: SSE-driven live task state changes
- **Task detail page**: Description (markdown), status, retries, logs, agent status, cost, dependencies, PR link, acceptance criteria, prerequisite failures
- **Epic detail page**: Proposed tasks with inline editing, planning session log, confirm/close actions, worker status indicators
- **Create task dialog**: Description, acceptance criteria, dependency search/selection, max cost budget, model selection
- **Create epic dialog**: Title, description, optional planning prompt
- **Settings management**: Server-wide default model configuration via API and UI
- **Task cards**: Preview with retry count, cost, dependency count, consecutive failure warnings
- **Repository management**: Selector dropdown, add from GitHub with search, remove repos
- **Close task**: Dialog with optional reason
- **Sync PRs**: Manual sync button with result summary

## Security & Isolation

- **User code stays on-premise**: Only task descriptions flow in; logs and PR notifications flow out
- **Docker container isolation**: Each agent runs in its own ephemeral container
- **Encrypted token storage**: GitHub tokens encrypted at rest using AES-256-GCM; managed via API (`PUT /settings/github-token`) instead of environment variables
- **Centralized credential management**: GitHub token stored encrypted in the database; workers receive it per-task over HTTPS
- **Worker authentication**: Per-user API keys for worker-to-server communication
