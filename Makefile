.PHONY: help dev dev-api dev-web web build image lint test tidy clean

VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/tm4rtin17/controlroom/internal/buildinfo.Version=$(VERSION) \
	-X github.com/tm4rtin17/controlroom/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/tm4rtin17/controlroom/internal/buildinfo.Date=$(DATE)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "} {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

dev: ## Print dev instructions
	@echo "Run these in two terminals:"
	@echo "  make dev-api   # Go backend with hot reload (requires air)"
	@echo "  make dev-web   # Vite dev server (requires Node 20+)"

dev-api: ## Run Go backend with hot reload (HTTP, dev cookies)
	@command -v air >/dev/null || { echo "install: go install github.com/air-verse/air@latest"; exit 1; }
	@mkdir -p .air/data
	CR_DEV=true CR_ADDR=:8443 CR_DATA_DIR=$(PWD)/.air/data CR_LOG_LEVEL=debug air -c .air.toml

dev-web: ## Run Vite dev server
	cd web && (test -d node_modules || npm install) && npm run dev

web: ## Build the frontend bundle into internal/web/dist/
	cd web && (test -f package-lock.json && npm ci || npm install) && npm run build

build: web ## Build the controlroom binary (writes ./controlroom)
	CGO_ENABLED=0 go build -trimpath -ldflags='$(LDFLAGS)' -o controlroom ./cmd/controlroom

image: ## Build the Docker image as controlroom:dev
	docker build -t controlroom:dev \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-f deploy/Dockerfile .

lint: ## Lint Go and frontend
	@command -v golangci-lint >/dev/null || { echo "install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...
	cd web && npm run lint

test: ## Run Go tests
	go test -race ./...

tidy: ## Tidy go.mod
	go mod tidy

clean: ## Remove build artifacts
	rm -f controlroom
	rm -rf .air web/node_modules web/.vite
	find internal/web/dist -mindepth 1 ! -name index.html -exec rm -rf {} + 2>/dev/null || true
