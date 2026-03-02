# Verve

An AI agent orchestrator that dispatches [Claude Code](https://docs.anthropic.com/en/docs/claude-code) agents to work on
tasks in isolated Docker containers. User source code never leaves their network.

## How It Works

```
Your Cloud                              User Environment
┌───────────────────────────┐          ┌───────────────────────────┐
│ Postgres ◄─► API Server   │◄─ HTTPS ─│ Worker                    │
│              ◄─► Web UI   │          │   └─► Agent containers    │
└───────────────────────────┘          └───────────────────────────┘
```

1. You create a task in the web UI with a description and target repository
2. A worker (running in the user's environment) long-polls and claims the task
3. The worker spawns an isolated Docker container running Claude Code
4. The agent makes code changes, commits, and opens a pull request
5. Logs stream back in real-time and PR status is monitored automatically
6. If CI fails, the task is automatically retried with failure context

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/joshjon/verve/main/scripts/install.sh | sh
```

Or install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/joshjon/verve/main/scripts/install.sh | sh -s 0.1.0
```

By default this installs to `~/.local/bin/verve`. Set `INSTALL_DIR` to override:

```bash
curl -fsSL https://raw.githubusercontent.com/joshjon/verve/main/scripts/install.sh | INSTALL_DIR=/usr/local/bin sh
```

## Quick Start

### Prerequisites

- Docker
- [Anthropic API Key](https://console.anthropic.com) (or Claude Code OAuth token)

### 1. Build the agent image and start Verve

```bash
make build-agent  # Build the agent Docker image
ANTHROPIC_API_KEY=sk-... verve
```

This starts the API server and worker together, with the UI at [http://localhost:7400](http://localhost:7400).

### 2. Add a repository and create a task

Open the dashboard, connect a GitHub repo, and create your first task.

### Docker Compose (distributed mode)

For production deployments with PostgreSQL:

```bash
cp .env.example .env   # Fill in your keys
make up                # Start PostgreSQL, API server, and worker
```

### Useful commands

```bash
make logs    # Tail container logs
make down    # Stop everything
```

## Custom Agent Images

The base agent image includes Node.js and common tools. If your project needs additional dependencies (Go, Python, Rust,
etc.), create a custom Dockerfile:

```dockerfile
FROM ghcr.io/joshjon/verve:base

USER root
COPY --from=golang:1.25-alpine /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
USER agent
```

See [`agent/examples/`](agent/examples/) for more examples.

## Tech Stack

- **Go** — API server and worker
- **SvelteKit** — Web UI
- **PostgreSQL** / SQLite — Database (Postgres for production, SQLite in-memory for dev)
- **Docker** — Agent container isolation
- **Claude Code** — AI coding agent
