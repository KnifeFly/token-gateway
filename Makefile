SHELL := /bin/sh

CONFIG ?= configs/local.yaml

.PHONY: help test lint fmt fmt-check vet race build run-gateway migrate-up migrate-down compose-up compose-down

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

migrate-up: ## Apply MySQL migrations
	go run ./cmd/migrate -config $(CONFIG) -direction up

migrate-down: ## Revert latest MySQL migration
	go run ./cmd/migrate -config $(CONFIG) -direction down

compose-up: ## Start local dependencies
	docker compose up -d mysql redis

compose-down: ## Stop local dependencies
	docker compose down
