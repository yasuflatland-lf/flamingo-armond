.DEFAULT_GOAL := help

.PHONY: help setup notice-prereqs check-docker mise-install supabase-restart supabase-stop sync-env install supabase-start check-google-oauth dev dev-backend dev-frontend codegen test clean clean-frontend clean-backend doctor

# Most env / Supabase targets dispatch to the playbook below; tags select the subset.
ANSIBLE := ansible-playbook -i playbooks/inventory.local playbooks/setup.yml

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: notice-prereqs check-docker mise-install ## One-shot initial setup: pnpm + supabase + sync env + google check (via Ansible)
	@$(ANSIBLE)

notice-prereqs: ## Up-front notice about manual prerequisites that run in parallel with setup
	@echo ""
	@echo "NOTE: Before this finishes you'll need a Google OAuth client for LOCAL dev."
	@echo "      While Docker pulls images, you can create one in parallel:"
	@echo "        -> docs/dev-setup.md section 'Supabase CLI' -> 'Auth flow (read this first)'"
	@echo "      (Production uses a SEPARATE OAuth client managed via Terraform; see"
	@echo "       docs/deployment.md section 'Manual prerequisites' -> Google OAuth client.)"
	@echo ""

install: ## Install pnpm workspace dependencies (Ansible-managed for change detection)
	@$(ANSIBLE) --tags pnpm

supabase-start: ## Boot local Supabase (no-op if already running, via Ansible)
	@$(ANSIBLE) --tags supabase

supabase-stop: ## Stop local Supabase (preserves volumes; add --no-backup manually to wipe data)
	supabase stop

supabase-restart: ## Stop and re-boot local Supabase (use after editing .env Google credentials)
	-supabase stop
	@$(ANSIBLE) --tags supabase

sync-env: ## Idempotently sync .env (root) + frontend/backend .env.local using marker-aware ownership
	@$(ANSIBLE) --tags sync-env

check-google-oauth: ## Verify Google OAuth credentials are set in root .env (warns if placeholder/missing)
	@$(ANSIBLE) --tags google-check

check-docker: ## Verify the Docker daemon is reachable (required by supabase start)
	@docker info >/dev/null 2>&1 || { echo "ERROR: Docker daemon not reachable. Start Docker Desktop, then re-run."; exit 1; }

mise-install: ## Install pinned tools via mise (auto-trusts mise.toml; provisions Python + ansible-core)
	@mise trust mise.toml >/dev/null 2>&1 || true
	@mise install

dev: ## Run preflight + backend + frontend via mprocs (requires Supabase up)
	@mprocs

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

clean: clean-frontend clean-backend ## Remove build caches (safe; no process kill, no node_modules, no DB)

clean-frontend: ## Remove frontend Next.js build cache (.next)
	rm -rf frontend/.next

clean-backend: ## Remove Go build and test caches (does not touch the module cache)
	cd backend && go clean -testcache -cache

doctor: ## Show which processes hold dev ports 1323/3000 (does NOT kill; you decide)
	@echo "Processes holding :1323 (backend):"
	@lsof -nP -i :1323 || echo "  (none)"
	@echo ""
	@echo "Processes holding :3000 (frontend):"
	@lsof -nP -i :3000 || echo "  (none)"
	@echo ""
	@echo "If a stale dev server is listed above, kill it manually: kill <PID>"
