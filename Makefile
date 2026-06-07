SHELL := /bin/sh

CONFIG ?= configs/local.yaml

.PHONY: help test lint fmt fmt-check vet race build run-gateway run-console run-worker loadtest portal-smoke portal-web-smoke p22-console-smoke p24-console-smoke release-handoff release-handoff-check failure-drills api-generate api-check boundary-check p24-cut-scope-check web-install web-lint web-typecheck web-test web-build migrate-up migrate-down compose-up compose-down

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "%-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run unit tests
	go test ./...

fmt: ## Format Go code
	@files=$$(find . -name '*.go' -not -path './.git/*'); \
	if [ -n "$$files" ]; then gofmt -w $$files; fi

fmt-check: ## Check Go formatting
	@files=$$(find . -name '*.go' -not -path './.git/*'); \
	if [ -n "$$files" ]; then \
		unformatted=$$(gofmt -l $$files); \
		if [ -n "$$unformatted" ]; then echo "$$unformatted"; exit 1; fi; \
	fi

vet: ## Run go vet
	go vet ./...

lint: fmt-check vet ## Run lightweight lint checks

race: ## Run tests with race detector
	go test -race ./...

build: ## Build all commands
	go build ./cmd/...

run-gateway: ## Run local gateway
	go run ./cmd/gateway -config $(CONFIG)

run-console: ## Run local console
	go run ./cmd/console -config $(CONFIG)

run-worker: ## Run local worker
	go run ./cmd/worker -config $(CONFIG)

api-generate: ## Generate frontend API types from split OpenAPI
	api/scripts/generate_ts_client.sh

api-check: ## Check split OpenAPI generated client drift
	api/scripts/check_api_diff.sh

boundary-check: ## Check P23 import and app/package boundaries
	scripts/check_import_boundaries.sh

p24-cut-scope-check: ## Check P24 lean console non-goals stay out of API/UI routes and fields
	scripts/check_p24_cut_scope.sh

web-install: ## Install frontend workspace dependencies
	pnpm install --frozen-lockfile

web-lint: ## Run frontend lint
	pnpm lint

web-typecheck: ## Run frontend typecheck
	pnpm typecheck

web-test: ## Run frontend tests
	pnpm test

web-build: ## Build frontend apps and packages
	pnpm build

loadtest: ## Run local M7 load test
	go run ./tools/loadtest

portal-smoke: ## Run P9 Portal customer smoke against GATEWAY_URL/API_KEY
	go run ./tools/portal-smoke

portal-web-smoke: ## Run P20 Portal Web BFF smoke against CONSOLE_URL/API_KEY
	go run ./tools/portal-web-smoke

p22-console-smoke: ## Run P22 Console production smoke against CONSOLE_URL/API_KEY/ADMIN_EMAIL/ADMIN_PASSWORD
	go run ./tools/p22-console-smoke

p24-console-smoke: ## Run P24 Admin/Portal console smoke against CONSOLE_URL/API_KEY/ADMIN_EMAIL/ADMIN_PASSWORD
	go run ./tools/p24-console-smoke

release-handoff: ## Print P10 release handoff document
	go run ./tools/release-handoff

release-handoff-check: ## Run P10 release checks and print handoff evidence
	go run ./tools/release-handoff -run-checks

failure-drills: ## Run M7 failure drills against GATEWAY_URL/API_KEY
	tests/failure/drills.sh

migrate-up: ## Apply MySQL migrations
	go run ./cmd/migrate -config $(CONFIG) -direction up

migrate-down: ## Revert latest MySQL migration
	go run ./cmd/migrate -config $(CONFIG) -direction down

compose-up: ## Start local dependencies
	docker compose up -d mysql redis

compose-down: ## Stop local dependencies
	docker compose down
