.DEFAULT_GOAL := help

.PHONY: help dev-backend dev-frontend codegen install test

help: ## このヘルプを表示
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## ルートで pnpm install（frontend workspace 含む）
	pnpm install

dev-backend: ## backend の dev server を 1323 で起動
	cd backend && go run ./cmd/server

dev-frontend: ## frontend の Next.js dev server を起動（PR3 以降で実装）
	pnpm --filter frontend dev

codegen: ## gqlgen + graphql-codegen を両方実行（PR2 / PR5 で実体化）
	cd backend && go tool gqlgen generate
	pnpm --filter frontend codegen

test: ## backend go test + frontend vitest
	cd backend && go test -race -covermode=atomic ./...
	pnpm --filter frontend test
