.DEFAULT_GOAL := help

.PHONY: help dev-backend dev-frontend codegen install test

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dependencies via pnpm at the repo root (includes the frontend workspace)
	pnpm install

dev-backend: ## Run the backend dev server on port 1323
	cd backend && go run ./cmd/server

dev-frontend: ## Run the frontend Next.js dev server
	pnpm --filter frontend dev

codegen: ## Run gqlgen (backend) and graphql-codegen (frontend)
	cd backend && go tool gqlgen generate
	pnpm --filter frontend codegen

test: ## Run backend go test and frontend vitest
	cd backend && go test -race -covermode=atomic ./...
	pnpm --filter frontend test
