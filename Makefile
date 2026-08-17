# StarLens development Makefile.
#
# `make run` starts the Go API and the Vite dev server together; Ctrl-C stops
# both. Ports can be overridden per invocation:
#
#   make run BACKEND_PORT=9090 FRONTEND_PORT=3000
#
# The backend reads its StarRocks connection from the environment; `make run`
# auto-loads backend/.env when present (copy backend/.env.example to start).

BACKEND_PORT  ?= 8080
FRONTEND_PORT ?= 5173

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
BACKEND_BIN  := $(BACKEND_DIR)/bin/starlens-server

STARROCKS_CONTAINER := starlens-starrocks

.DEFAULT_GOAL := help

# ---------------------------------------------------------------- run --------

.PHONY: run
run: $(FRONTEND_DIR)/node_modules ## Run API + web dev server together (Ctrl-C stops both)
	@echo "StarLens dev: API on :$(BACKEND_PORT), web on :$(FRONTEND_PORT)"
	@trap 'kill 0 2>/dev/null' INT TERM EXIT; \
	{ $(MAKE) --no-print-directory run-backend; kill 0 2>/dev/null; } & \
	{ $(MAKE) --no-print-directory run-frontend; kill 0 2>/dev/null; } & \
	wait

.PHONY: run-backend
run-backend: ## Run only the Go API server (auto-loads backend/.env)
	@cd $(BACKEND_DIR) && set -a; [ -f .env ] && . ./.env; set +a; \
	SERVER_PORT=$(BACKEND_PORT) exec go run ./cmd/server

.PHONY: run-frontend
run-frontend: $(FRONTEND_DIR)/node_modules ## Run only the Vite dev server
	@cd $(FRONTEND_DIR) && VITE_PROXY_TARGET=http://localhost:$(BACKEND_PORT) \
	exec npm run dev -- --port $(FRONTEND_PORT) --strictPort

# -------------------------------------------------------------- build --------

.PHONY: build
build: build-backend build-frontend ## Build the API binary and the production web bundle

.PHONY: build-backend
build-backend: ## Compile the API server to backend/bin/starlens-server
	cd $(BACKEND_DIR) && go build -o bin/starlens-server ./cmd/server

.PHONY: build-frontend
build-frontend: $(FRONTEND_DIR)/node_modules ## Type-check and bundle the web app to frontend/dist
	cd $(FRONTEND_DIR) && npm run build

# --------------------------------------------------------------- test --------

.PHONY: test
test: ## Run backend unit tests (the frontend has no test suite yet)
	cd $(BACKEND_DIR) && go test ./...

# ------------------------------------------------------------ quality --------

.PHONY: lint
lint: $(FRONTEND_DIR)/node_modules ## gofmt check + go vet + oxlint
	@unformatted=$$(gofmt -l $(BACKEND_DIR)); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; echo "Run 'make fmt'."; exit 1; \
	fi
	cd $(BACKEND_DIR) && go vet ./...
	cd $(FRONTEND_DIR) && npm run lint

.PHONY: fmt
fmt: ## Format Go sources in place
	gofmt -w $(BACKEND_DIR)

.PHONY: check
check: lint test build ## Everything CI would run: lint, tests, full build

# ------------------------------------------------------------ helpers --------

.PHONY: deps
deps: ## Install/refresh all dependencies (go mod + npm)
	cd $(BACKEND_DIR) && go mod tidy
	npm --prefix $(FRONTEND_DIR) install

# npm install is driven by package.json so a plain checkout bootstraps itself.
$(FRONTEND_DIR)/node_modules: $(FRONTEND_DIR)/package.json
	npm --prefix $(FRONTEND_DIR) install
	@touch $@

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BACKEND_DIR)/bin $(FRONTEND_DIR)/dist

.PHONY: starrocks-up
starrocks-up: ## Start a local single-node StarRocks in Docker (FE query port 9030)
	@docker start $(STARROCKS_CONTAINER) 2>/dev/null || \
	docker run -d --name $(STARROCKS_CONTAINER) \
		-p 9030:9030 -p 8030:8030 -p 8040:8040 \
		starrocks/allin1-ubuntu
	@echo "StarRocks is starting (takes ~60s to become ready)."
	@echo "Default DSN: root:@tcp(127.0.0.1:9030)/information_schema"

.PHONY: starrocks-down
starrocks-down: ## Stop the local StarRocks container (data kept; 'docker rm -f starlens-starrocks' to delete)
	docker stop $(STARROCKS_CONTAINER)

.PHONY: help
help: ## Show this help
	@echo "StarLens — available targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
