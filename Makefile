AGENT_IMAGE = ghcr.io/joshjon/verve

# ── Agent Images ─────────────────────────────────────────────

.PHONY: build-agent
build-agent:
	docker build -t verve:base ./agent

.PHONY: build-agent-dev
build-agent-dev: build-agent
	docker build -f agent/Dockerfile.dev -t verve:dev ./agent

.PHONY: push-agent
push-agent: build-agent
ifdef TAG
	docker tag verve:base $(AGENT_IMAGE):$(TAG)
	docker tag verve:base $(AGENT_IMAGE):base
	docker push $(AGENT_IMAGE):$(TAG)
	docker push $(AGENT_IMAGE):base
else
	docker tag verve:base $(AGENT_IMAGE):base
	docker push $(AGENT_IMAGE):base
endif

# ── Code Generation ──────────────────────────────────────────

.PHONY: generate
generate:
	go generate ./internal/postgres/... ./internal/sqlite/...

# ── UI ───────────────────────────────────────────────────────

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

# ── Docker Compose ───────────────────────────────────────────

.PHONY: up
up: build-agent-dev
	docker compose up -d

.PHONY: up-build
up-build: build-agent-dev
	docker compose up -d --build

.PHONY: down
down:
	docker compose down

.PHONY: logs
logs:
	docker compose logs -f

# ── Release & Deploy ─────────────────────────────────────────

.PHONY: release
release:
	./scripts/release.sh $(BUMP)

.PHONY: deploy
deploy: ui-build-go
	fly deploy --config deploy/fly.toml --local-only

# ── Cleanup ──────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -rf bin/
	rm -rf ui/dist/
	docker rmi verve:base 2>/dev/null || true
