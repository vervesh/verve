AGENT_IMAGE = ghcr.io/joshjon/verve-agent

# Build all components
.PHONY: all
all: build-agent build

# Build Go binary
.PHONY: build
build:
	go build -o bin/verve .

# Build agent Docker image
.PHONY: build-agent
build-agent:
	docker build -t verve-agent:latest ./agent

.PHONY: build-agent-dev
build-agent-dev: build-agent
	docker build -f agent/Dockerfile.dev -t verve-agent:dev ./agent

.PHONY: build-agent-no-cache
build-agent-no-cache:
	docker build --no-cache -t verve-agent:latest ./agent

# Push agent image to GitHub Container Registry
# Usage:
#   make push-agent              # pushes :latest
#   make push-agent TAG=v0.1.0   # pushes :v0.1.0 and :latest
.PHONY: push-agent
push-agent: build-agent
ifdef TAG
	docker tag verve-agent:latest $(AGENT_IMAGE):$(TAG)
	docker tag verve-agent:latest $(AGENT_IMAGE):latest
	docker push $(AGENT_IMAGE):$(TAG)
	docker push $(AGENT_IMAGE):latest
else
	docker tag verve-agent:latest $(AGENT_IMAGE):latest
	docker push $(AGENT_IMAGE):latest
endif

# Lint
.PHONY: lint
lint:
	golangci-lint run ./...

# Generate sqlc code
.PHONY: generate
generate:
	go generate ./internal/postgres/... ./internal/sqlite/...

# Run locally — starts both API server + worker
.PHONY: run
run: build
	./bin/verve

# Run API server only
.PHONY: run-api
run-api: build
	./bin/verve api

# Run API server with PostgreSQL
.PHONY: run-api-pg
run-api-pg: build
	docker compose up -d postgres
	@echo "Waiting for postgres..."
	@sleep 2
	POSTGRES_USER=verve POSTGRES_PASSWORD=verve POSTGRES_HOST_PORT=localhost:5432 POSTGRES_DATABASE=verve ./bin/verve api

# Run worker only
.PHONY: run-worker
run-worker: build
	./bin/verve worker

# Docker Compose (full stack)
.PHONY: up
up:
	docker compose up -d

.PHONY: up-build
up-build:
	docker compose up -d --build

.PHONY: down
down:
	docker compose down

.PHONY: logs
logs:
	docker compose logs -f

# Docker Compose (dev stack — uses verve-agent:dev)
.PHONY: dev
dev: build-agent-dev
	docker compose -f docker-compose.dev.yml up -d

.PHONY: dev-build
dev-build: build-agent-dev
	docker compose -f docker-compose.dev.yml up -d --build

.PHONY: dev-down
dev-down:
	docker compose -f docker-compose.dev.yml down

.PHONY: dev-logs
dev-logs:
	docker compose -f docker-compose.dev.yml logs -f

# Create a test task
.PHONY: test-task
test-task:
	curl -X POST http://localhost:7400/api/v1/tasks \
		-H "Content-Type: application/json" \
		-d '{"description":"Init project with hello world main function using a plain bash script"}'

# List all tasks
.PHONY: list-tasks
list-tasks:
	curl -s http://localhost:7400/api/v1/tasks | jq .

# Get task by ID (usage: make get-task ID=tsk_xxx)
.PHONY: get-task
get-task:
	curl -s http://localhost:7400/api/v1/tasks/$(ID) | jq .

# Tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/
	rm -rf ui/dist/
	docker rmi verve-agent:latest 2>/dev/null || true

# UI commands
.PHONY: ui-install
ui-install:
	cd ui && pnpm install

.PHONY: ui-dev
ui-dev:
	cd ui && pnpm dev

.PHONY: ui-build
ui-build:
	cd ui && pnpm build

.PHONY: ui-build-go
ui-build-go:
	cd ui && BUILD_PATH="../internal/frontend/dist" VITE_API_URL="" pnpm build
	git checkout -- internal/frontend/dist/placeholder.html

# Release — tag and publish via goreleaser
# Usage:
#   make release           # patch bump (default)
#   make release BUMP=minor
#   make release BUMP=major
.PHONY: release
release:
	./scripts/release.sh $(BUMP)

.PHONY: install-dev
install-dev:
	GOBIN=$(HOME)/.local/bin go install github.com/joshjon/verve@main

# Deploy to Fly.io (builds UI into Go binary, then deploys)
.PHONY: deploy
deploy: ui-build-go
	fly deploy --config deploy/fly.toml --local-only
