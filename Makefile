.DEFAULT_GOAL := help

.PHONY: help setup notice-prereqs check-docker mise-install supabase-restart supabase-stop sync-env install supabase-start check-google-oauth dev dev-backend dev-frontend codegen codegen-yacc test clean clean-frontend clean-backend doctor db-reset setup-prod setup-prod-preflight setup-prod-postapply teardown-prod teardown-prod-preflight seed-admin sync-notion-secrets sync-notion-preflight notion-local-setup notion-local-run

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
	@echo "      (Production uses a SEPARATE OAuth client; see docs/deployment.md"
	@echo "       section 'Manual prerequisites' -> Google OAuth client.)"
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

db-reset: ## DESTRUCTIVE: drop and re-create local Supabase DB (re-runs migrations + seed)
	supabase db reset

seed-admin: ## Grant admin role to EMAIL=<address> via local Supabase psql (idempotent; one-shot fallback)
	@if [ -z "$(EMAIL)" ]; then echo "ERROR: EMAIL is required, e.g. make seed-admin EMAIL=you@example.com"; exit 1; fi
	@$(ANSIBLE) --tags seed-admin -e "admin_email=$(EMAIL)"

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

.PHONY: codegen-yacc
codegen-yacc: ## Regenerate the goyacc-driven dictionary parser
	cd backend && go tool goyacc -o internal/textdic/parser.go -p yy internal/textdic/grammar.y
	rm -f backend/y.output y.output

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

# --- Production bring-up (manual runbook + verification) -------------------
ANSIBLE_PROD := ansible-playbook -i playbooks/inventory.local playbooks/setup-prod.yml
ANSIBLE_TEARDOWN := ansible-playbook -i playbooks/inventory.local playbooks/teardown-prod.yml

setup-prod: mise-install ## Guided production bring-up: prereq check + dashboard handoff + smoke
	@$(ANSIBLE_PROD)

# `confirm=true` skips the Google OAuth reminder pause so this stays a true
# unattended scanner. Operators still see the reminder on a full `make setup-prod`.
setup-prod-preflight: mise-install ## Verify tokens and GitHub App installations only (no operator handoff)
	@$(ANSIBLE_PROD) --tags preflight -e confirm=true

setup-prod-postapply: mise-install ## Trigger first Render deploy + smoke tests (re-runnable from .state.yml)
	@$(ANSIBLE_PROD) --tags postapply

sync-notion-secrets: mise-install ## Sync NOTION_* to Render env + GHA secrets (idempotent)
	@$(ANSIBLE_PROD) --tags notion

sync-notion-preflight: mise-install ## Verify root .env has all required NOTION_* without writing
	@$(ANSIBLE_PROD) --tags notion-preflight

notion-local-setup: mise-install ## Write NOTION_* to backend/.env.local from root .env (run once, or after editing NOTION_LOCAL_*)
	@$(ANSIBLE) --tags notion-local

notion-local-run: mise-install ## Trigger local /internal/notion-sync (requires `make dev-backend` running in another terminal)
	@scripts/notion-local-run.sh

teardown-prod: mise-install ## DESTRUCTIVE: tear down the production environment created by setup-prod
	@$(ANSIBLE_TEARDOWN)

teardown-prod-preflight: mise-install ## Verify tokens and resolve IDs only (no destructive work)
	@$(ANSIBLE_TEARDOWN) --tags preflight
