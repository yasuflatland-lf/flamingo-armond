# Improvement master plan: porting features from the old implementation

> Created: 2026-04-30
> Author: feature-dev workflow run on `feature/ping_services`
> Source repo: `/Users/yasuflatland/tmp/flamingo-armond-old`
> Target repo: `/Users/yasuflatland/projects/flamingo-armond` (current)
> Output convention: this is the **master plan**. Each PR gets its own implementation plan as a GitHub Issue, derived from the corresponding section below.

---

## 1. Goal

Take the legacy features that exist in the old implementation and bring them into the new repository, **redesigned to fit the new architecture** (DDD aggregates, gqlgen, DataLoader, Supabase JWT, golang-migrate, RLS-enabled Postgres, Echo v5, Next.js 16 App Router, Apollo + graphql-codegen).

The output of executing this plan is a set of **13 PRs**, each ≈800 lines of production code (tests excluded from that count), landing in dependency order on `main`.

## 2. Scope

In scope:

- Admin frontend, RBAC enforcement, dictionary bulk import.
- Card features: bulk delete, FSRS field override on creation, adaptive learning.
- Card list pagination (Relay Connection).
- Adaptive learning / performance mode (replace the `Always 0` placeholder).
- Test revival: Vitest page tests + Playwright E2E + Vercel deploy CI.
- Dev seed data (`supabase/seed.sql`).
- RLS policies (owner + admin bypass).

Explicitly out of scope:

- `docker-compose.yml` (the new repo intentionally uses `mprocs` + Supabase CLI; do not reintroduce).
- Production seed (manual via Supabase + Ansible playbook; runbook only, no code).
- Reintroducing goose migrations (we keep `golang-migrate`).
- Reintroducing the legacy hand-rolled JWT auth (we use Supabase JWT via JWKS).
- `ProtectedRoute` React HOC verbatim — replaced by Supabase middleware + server-side RLS.
- Real-time Subscriptions (the old repo did not have them either; out of scope).

## 3. Decisions confirmed before planning

| ID  | Topic                       | Decision                                                                                                  |
| --- | --------------------------- | --------------------------------------------------------------------------------------------------------- |
| Q1  | RBAC source of truth        | **B**: DB tables (`roles`, `user_roles`) are authoritative; RLS uses `is_admin()` SQL helper.             |
| Q2  | Admin scope                 | **c**: users + roles + dictionary bulk import.                                                            |
| Q3  | Dictionary parser           | **a**: keep `goyacc`, register it via `backend/tools.go` so it runs through `go tool`; Makefile codegen.  |
| Q4  | Adaptive learning scope     | **c**: ship the calculation **and** the UI surfaces in one go.                                            |
| Q5  | Card list pagination shape  | **a**: Relay-style Connection (`first/after/last/before`, `edges/nodes/pageInfo/totalCount`).             |
| Q6  | E2E strategy                | **c**: Vitest for component/page tests + Playwright for top-flow E2E. Phased.                             |
| Q7  | docker-compose.yml          | **b**: do not reintroduce. Playwright will use its own `webServer` config when needed.                    |
| Q8  | Seed data                   | **a**: `supabase/seed.sql` only (dev). No prod seed code.                                                 |
| Q9  | RLS policy posture          | **c**: owner-based + admin bypass via `is_admin(auth.uid())`.                                             |
| Q10 | PR ordering                 | Foundation → features → tests/CI → seed (last). See § 7.                                                  |
| Q11 | `ping_records` migration    | Already exists (`20260501000000_add_ping_records.up.sql`). No PR-00 required.                             |
| Q12 | Plan output location        | This file (`.claude/plans/improve_master_plan.md`).                                                       |

## 4. Architectural posture

### 4.1 Vertical-slice + foundation hybrid

- **Foundation PRs** (PR-01, PR-02) are backend-only and ship the helpers + RLS that downstream PRs depend on.
- **Feature PRs** (PR-03 through PR-09) are vertical slices: schema → migration → repo → usecase → resolver → frontend page → tests, all in one PR. CI is path-scoped (`backend/**` / `frontend/**`) so vertical slices trip both jobs but each job is fast (~45-60 s).
- **Quality PRs** (PR-10, PR-11, PR-12) wire deploy and test infrastructure.
- **Polish PR** (PR-13) seeds dev data once everything else exists.

### 4.2 Where new things land (canonical locations)

| Layer                     | Path                                                       | Pattern                                                          |
| ------------------------- | ---------------------------------------------------------- | ---------------------------------------------------------------- |
| GraphQL schema            | `schema/schema.graphql`                                    | One file. Use `extend type Query/Mutation`. See `:135` and `:142` for the existing pattern. |
| Backend domain            | `backend/internal/domain/`                                 | One file per aggregate root. Value objects in same file.         |
| Backend domain services   | `backend/internal/domain/service/`                         | Stateless calculators (e.g., `FSRSScheduler`, future `UserPerformanceService`). |
| Repository interfaces     | `backend/internal/repository/<aggregate>.go`               | Interface + GORM impl + `xxxTx` variants for transactional paths. |
| Usecases                  | `backend/internal/usecase/<aggregate>.go`                  | Owner check via `auth.UserFrom(ctx)`. Tx wraps per-method.       |
| DataLoaders               | `backend/internal/loader/<entity>.go`                      | Add struct field in `loader.go` + `New()`.                       |
| Resolver wiring           | `backend/graph/resolver/resolver.go`                       | Append `<Aggregate>UC` field; never collide with type names.     |
| Resolver methods          | `backend/graph/resolver/schema.resolvers.go`               | Re-generated by `gqlgen`; manual helpers go **outside** the managed block. |
| Migrations                | `backend/internal/database/migrations/`                    | `<ts>_<slug>.{up,down}.sql`. `golang-migrate` autoruns at boot.  |
| Auth/RBAC helpers         | `backend/internal/auth/`                                   | New `role.go` for `IsAdmin`; reuse `auth.UserFrom(ctx)`.         |
| GQL error codes           | `backend/internal/gqlerr/errors.go`                        | Add `FORBIDDEN` constructor here.                                |
| Frontend pages            | `frontend/src/app/<segment>/page.tsx`                      | RSC by default. Use `gqlFetch` server-side, `ApolloClient` client-side. |
| Frontend admin pages      | `frontend/src/app/admin/...`                               | New segment. Layout enforces admin via server-side `me` check.   |
| Frontend GraphQL docs     | `frontend/src/app/<segment>/queries.ts`                    | One file per segment. `graphql()` returns TypedDocumentNode.     |
| Frontend tests            | `frontend/__tests__/<segment>.test.tsx`                    | Vitest + Testing Library. (New convention; see PR-11.)           |
| E2E tests                 | `frontend/e2e/<flow>.spec.ts`                              | Playwright. (New convention; see PR-12.)                         |
| Dev seed                  | `supabase/seed.sql`                                        | New file. Runs on `supabase db reset`. Dev only.                 |
| CI                        | `.github/workflows/`                                       | Modify `frontend.yml` for Vercel; new `e2e.yml` for Playwright.  |

### 4.3 Diff vs the old codebase

The following old-→new deltas are non-negotiable. Every PR must respect them:

1. **`gorm.Model` → explicit columns**. Old code uses GORM's embedded `gorm.Model`; new code uses explicit `created_at` / `updated_at` columns and triggers.
2. **`time.Time` → `timestamptz`**. Trigger-set `updated_at` already exists for users / cardgroups / cards.
3. **goose → golang-migrate**. Filename format `<ts>_<slug>.{up,down}.sql` (no `+goose` markers). Embedded via `embed.FS`.
4. **REST helpers (`/lib/api/*.ts`) → GraphQL via gqlFetch / ApolloClient**. The old `usersApi`, `authApi` files are not ported.
5. **`AuthContext` + cookie JWT → Supabase SSR client**. The old `ProtectedRoute` is replaced by the existing `frontend/src/middleware.ts` + server-side `me` checks.
6. **Echo v4 (`echo.Context`) → Echo v5 (`*echo.Context`)**. All ported handlers must convert.
7. **`fmt.Errorf("%w", err)` → `eris.Wrap(err, ...)`**. CI greps for the former.
8. **CJK characters → English only** in committed code, comments, docs, commit messages, PR titles. (See `.claude/rules/language-policy.md`.)

## 5. Cross-cutting rules per PR

Every PR must:

1. Stay within ≈800 lines of **production** code (tests excluded). If a feature exceeds, split.
2. Use Conventional Commits in title (`feat(backend): ...`, `feat(frontend): ...`, `feat(schema): ...`, etc.).
3. Re-run `go tool gqlgen generate` (backend) and `pnpm --filter frontend codegen` (frontend) after schema changes; both CIs verify.
4. Pass `go vet`, `go test -race`, `biome check`, `tsc --noEmit`, `vitest`, `pnpm build` locally before push.
5. Pass the language-policy verification (`grep -rlP "[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]" ...` and `grep -rnE "PR[0-9]+" ...`).
6. Add or update L2 docs (`docs/`) when introducing concepts; never inflate L1 (`CLAUDE.md`) past 35 lines.
7. Squash-merge to `main`.

## 6. PR overview table

| #     | Title                                              | Layer touched     | Depends on              | Prod LoC est. |
| ----- | -------------------------------------------------- | ----------------- | ----------------------- | ------------- |
| PR-01 | RBAC backend foundation **[in flight on `feature/improve_pr1`; impl + tests landed locally 2026-04-30, awaiting review/commit]** | be                | —                       | 150-250       |
| PR-02 | RLS policies (owner + admin bypass)                | be (SQL only)     | PR-01                   | 200-300       |
| PR-03 | Card list pagination (Relay Connection)            | be + fe           | PR-02                   | 600-800       |
| PR-04 | Card bulk delete + NewCard FSRS override           | be + fe           | PR-03                   | 500-700       |
| PR-05 | Adaptive learning (UserPerformanceService + UI)    | be + fe           | PR-04                   | 700-800       |
| PR-06 | Dictionary parser (goyacc) + validateDictionary    | be                | PR-04                   | 500-700       |
| PR-07 | upsertDictionary + admin import page               | be + fe (admin)   | PR-01, PR-06            | 600-800       |
| PR-08 | Admin: user list / edit / role assignment          | be + fe (admin)   | PR-01                   | 800-1000      |
| PR-09 | Admin: role CRUD + admin layout/nav                | be + fe (admin)   | PR-08                   | 600-800       |
| PR-10 | Vercel deploy CI job                               | ci                | —                       | 80-150        |
| PR-11 | Vitest page tests revival                          | fe (tests only)   | PR-03..PR-09            | 400-700       |
| PR-12 | Playwright E2E + workflow                          | fe + ci           | PR-09, PR-10 (PR-13 soft) | 500-700     |
| PR-13 | Dev seed (`supabase/seed.sql`)                     | sql               | PR-01..PR-09            | 150-250       |

Total: ~6,500–8,500 lines of production code across 13 PRs.

## 7. Per-PR detail

Each section below is the seed for the GitHub Issue / individual implementation plan.

---

### PR-01 — RBAC backend foundation

**Goal**: Wire admin role detection end-to-end inside the Go backend, without yet touching the frontend. After this PR a usecase can ask "is this user admin?" and reject if not.

**Why first**: Every other admin PR (PR-07, PR-08, PR-09) and the RLS PR (PR-02) call `is_admin()` either from SQL or from Go. Without this helper they cannot land cleanly.

**Existing assets** (already in the repo — do **not** re-create):
- `backend/internal/domain/role.go` — `Role` value object.
- `backend/internal/repository/role.go:20` — `RoleRepository` interface with `FindByName`, `FindByIDs`.
- `backend/internal/repository/user_role.go:22` — `UserRoleRepository.HasRole(ctx, userID, roleName)`.
- `backend/internal/loader/loader.go:17` — `Role` DataLoader registered in `Loaders` and constructed in `New()`.
- `public.roles` and `public.user_roles` tables, seeded with `admin` role.

**In scope** (the slim delta on top of the existing assets):
- New migration `<ts>_add_rbac_helpers.up.sql` defining the `is_admin(uid uuid) returns boolean` SQL helper (`STABLE`, `SECURITY DEFINER`, `REVOKE ALL FROM PUBLIC`, `GRANT EXECUTE TO authenticated`). Body queries `user_roles ⋈ roles WHERE name = 'admin'`.
- New `backend/internal/auth/role.go`: `Service.IsAdmin(ctx, uid) (bool, error)` — thin wrapper that delegates to `UserRoleRepository.HasRole(ctx, uid, "admin")`.
- Extend `backend/internal/gqlerr/errors.go:21` with `CodeForbidden = "FORBIDDEN"` and `NewForbidden(msg string) *gqlerror.Error`.
- DI wiring in `backend/cmd/server/main.go` (`run()` around line 190): construct `UserRoleRepository`, pass it into `auth.Service`.

**Out of scope**:
- No GraphQL schema changes.
- No frontend changes.
- No `assignRole` / `revokeRole` Mutations or repo methods — those land in PR-08, where `RoleRepository` is extended with `AssignToUser`, `RevokeFromUser`, `ListByUser`.
- No RLS policies — PR-02.

**Anchor files**:
- `backend/internal/database/migrations/20260430080000_initial_schema.up.sql:62` (existing `roles` / `user_roles` tables).
- `backend/internal/repository/user_role.go:22` (`HasRole` to delegate to).
- `backend/internal/loader/loader.go:17` (existing Role DataLoader to leave unchanged).
- `backend/internal/auth/middleware.go` (Service struct to extend with `IsAdmin`).
- `backend/internal/gqlerr/errors.go:21` (where to add `FORBIDDEN`).
- `backend/cmd/server/main.go:190` (DI wiring point).

**DB impact**:
- Migration adds `is_admin(uuid) → boolean` function. No new tables.

**Tests** (not counted in production LoC):
- `backend/internal/auth/role_test.go`: `IsAdmin` true/false matrix.
- `backend/internal/database/is_admin_function_test.go`: SQL function returns expected values for seeded admin / non-admin UUIDs.

**Risks**:
- `is_admin()` is `SECURITY DEFINER`. `REVOKE ALL FROM PUBLIC` then `GRANT EXECUTE TO authenticated` is mandatory to prevent anonymous PostgREST callers from probing role membership.
- `STABLE` (not `IMMUTABLE`) — the function reads tables.

**Verification**:
- New tests pass under `go test -race ./...`.
- `psql ... -c "SELECT is_admin('<seed-admin-uuid>');"` returns `t`.

---

### PR-02 — RLS policies (owner + admin bypass)

**Goal**: Replace the default-deny-on-all-public-tables stance with concrete row-level policies for `users`, `cardgroups`, `cards`, `swipe_records`, `roles`, `user_roles`. Admins bypass via `is_admin(auth.uid())`.

**Why second**: Once RLS policies exist, PostgREST callers (anon, authenticated) can be re-enabled in the future without touching app code. Even though the Go backend bypasses RLS as the table owner, the policies are the security baseline if the schema is ever exposed via PostgREST or Supabase Edge Functions.

**In scope**:
- New migration `<ts>_add_rls_policies.up.sql` with policies per table:
  - `users`: SELECT/UPDATE if `id = auth.uid()` OR `is_admin(auth.uid())`.
  - `cardgroups`: ALL if `owner_id = auth.uid()` OR `is_admin(auth.uid())`.
  - `cards`: ALL if `cardgroup_id IN (SELECT id FROM public.cardgroups WHERE owner_id = auth.uid())` OR `is_admin(auth.uid())`. (No direct `user_id` column; goes through `cardgroups`.)
  - `swipe_records`: SELECT if `user_id = auth.uid()` OR `is_admin(auth.uid())`. INSERT only by `user_id = auth.uid()` (admins do not write swipes for others).
  - `roles`: SELECT public; INSERT/UPDATE/DELETE only `is_admin(auth.uid())`.
  - `user_roles`: SELECT if `user_id = auth.uid()` OR `is_admin(auth.uid())`. Mutations admin-only.
- Down migration drops every policy.
- Test infra update: backend integration tests should keep using the table-owner connection (which bypasses RLS); a single new test in `backend/internal/database/rls_test.go` connects as the `authenticated` role and exercises a happy + denied path per table.

**Out of scope**:
- No application code changes (Go backend is unaffected because it connects as the table owner).
- No `FORCE ROW LEVEL SECURITY` (we explicitly keep the bypass for the backend role).

**Anchor files**:
- `backend/internal/database/migrations/20260430080000_initial_schema.up.sql:182` (block where RLS is enabled with zero policies; this PR adds the policies).
- `backend/internal/database/database.go` (DB role setup; reference for the test that connects as `authenticated`).
- `docs/backend.md` "Authorization at the usecase layer" section (update to note "RLS now has concrete policies; backend still bypasses via owner role").

**DB impact**:
- One migration file with ~12 `CREATE POLICY` statements.
- `swipe_records` INSERT policy is the only INSERT-level rule; everything else is ALL/SELECT.

**Tests** (additional):
- `backend/internal/database/rls_test.go`: connect as `authenticated`, set `request.jwt.claims` to two test users, prove user-A cannot SELECT user-B's cards.

**Risks**:
- Forgetting `WITH CHECK` on `FOR ALL` policies leaves a write-bypass hole. Each policy must specify both `USING` and `WITH CHECK` (or the planner inherits `USING` for `WITH CHECK`, but be explicit).
- Setting `auth.uid()` from a test connection requires `SET LOCAL request.jwt.claims = '{"sub":"<uuid>"}'` and connecting as a JWT-authenticated role. The test infra must support this.

**Verification**:
- `psql -c "\d+ public.cards"` shows policies present.
- `rls_test.go` passes.
- Existing backend tests still pass (regression check).

**Implementation task order and progress**:

Tasks are ordered by impact high / change size low:

1. [x] Add the RLS policy migration and matching down migration.
   - Dependency: PR-01 `public.is_admin(uuid)` migration must already exist.
   - Blocks: authenticated-role integration tests.
2. [x] Add authenticated-role test infrastructure and RLS coverage.
   - Dependency: task 1 policy names and access rules are fixed.
   - Parallel group A after task 1: this task can run alongside task 3.
3. [x] Update backend docs to describe concrete RLS policies with owner-role backend bypass.
   - Dependency: task 1 policy posture is fixed.
   - Parallel group A after task 1: this task can run alongside task 2.
4. [x] Run backend verification and language-policy checks.
   - Dependency: tasks 1-3 complete.
5. [x] Commit in reviewable slices.
   - Dependency: task 4 complete.
   - Scope guard: PR-03 / issue #47 pagination work is next and must not be included here.

---

### PR-03 — Card list pagination (Relay Connection)

**Goal**: Replace the unbounded `cardsByCardgroup(cardgroupId: ID!): [Card!]!` query with a Relay-style Connection that supports infinite scroll. Wire the frontend cards-by-cardgroup page to fetch incrementally.

**Why now**: Foundation work is done. Pagination is the most independent feature (no admin / RBAC / dictionary dependency) and unblocks PR-04 (bulk delete needs the list to exist with selection).

**In scope**:

Schema (`schema/schema.graphql`):
- Add `CardConnection`, `CardEdge`, `PageInfo` types.
- Add Query: `cardsByCardgroupConnection(cardgroupId: ID!, first: Int, after: ID, last: Int, before: ID, orderBy: CardOrderBy = ID, orderDirection: SortOrder = ASC): CardConnection!`.
- Add enums: `enum CardOrderBy { ID, CREATED_AT, UPDATED_AT, DUE }` and `enum SortOrder { ASC, DESC }`.
- Keep `cardsByCardgroup` for now but mark with `@deprecated(reason: "Use cardsByCardgroupConnection")`.

Backend:
- New `repository/card.go` method `FindPageByCardgroup(ctx, cardgroupID string, after, before *Cursor, first, last int, orderBy CardOrderBy, dir SortOrder) ([]*domain.Card, totalCount int64, error)`. Cursor type: `type Cursor struct { ID string; Due *time.Time; CreatedAt *time.Time; UpdatedAt *time.Time }` (only the fields relevant to the active `orderBy` are populated; comparison uses lexicographic `(orderField, id)` tuples).
- `usecase/card.go` adds `ListCardsByCardgroupConnection` with owner check; reject invalid `orderBy` (allowlist).
- Cursor encoding: cursor = card UUID (no base64; UUID is already an opaque ID). `after: $cursor` translates to `WHERE id > $cursor` (or `<` for DESC).
- `totalCount` uses a separate `COUNT(*)` (acceptable for ≤ 10k cards/group; document trade-off).

Frontend:
- `frontend/src/app/cardgroups/[id]/cards/page.tsx`: convert from "fetch all" RSC into a Client Component using Apollo with `fetchMore`. Or keep RSC for first page and hydrate Client for next pages.
- Implement Intersection Observer at the bottom sentinel.
- `frontend/src/app/cardgroups/[id]/cards/queries.ts`: add the Connection document.

**Out of scope**:
- `allUserCards` is not needed yet (no UI surface).
- Frontend filter UI (sort selector). Defaults are fine.

**Anchor files**:
- `schema/schema.graphql:138-139` (current `cardsByCardgroup` to deprecate).
- `backend/internal/repository/card.go:40` (existing `FindByIDTx` / `FindDueCardsTx` patterns).
- `backend/internal/usecase/card.go:175` (`authorizeCardgroup` reuse).
- `frontend/src/app/cardgroups/[id]/cards/page.tsx` (current implementation).
- `frontend/src/lib/apollo/server.ts` (`gqlFetch` for first-page RSC fetch).
- `frontend/src/app/cardgroups/queries.ts:5` (queries.ts pattern).

**DB impact**:
- Index on `(cardgroup_id, id)` is implicit through PK; verify with `EXPLAIN ANALYZE`. If sorting by `due` becomes the default, add `(cardgroup_id, due, id)` index.

**Tests** (additional):
- `backend/internal/repository/card_pagination_test.go`: forward, backward, edge of empty group, edge of single page.
- `frontend/__tests__/cards-pagination.test.tsx`: triggers Intersection Observer, asserts second page fetched, asserts `hasNextPage=false` stops fetching. (Narrow test; broad list rendering is PR-11.)

**Risks**:
- `WHERE id > cursor` only works for deterministic ordering. If `orderBy=DUE`, ties on `due` need a secondary key (`(due, id)`); make sure the SQL composes `(due, id) > (cursor_due, cursor_id)`.
- Empty `IN ()` regression in `FindPageByCardgroup` if `before/after` are both empty.

**Verification**:
- New tests pass.
- Frontend dev: 30+ cards in a group scrolls smoothly.
- `EXPLAIN` shows index usage on `(cardgroup_id, id)` for the default order.

---

### PR-04 — Card bulk delete + NewCard FSRS override

**Goal**: Add `deleteCards(ids: [ID!]!): Int!` mutation and extend `NewCardInput` with optional FSRS fields (used at dictionary import time to seed cards with non-default schedules).

**Why now**: Bulk delete depends on the list (PR-03) for selection UX. FSRS override is a precondition for PR-06 / PR-07 (dictionary import wants to set explicit defaults).

**In scope**:

Schema:
- `extend type Mutation { deleteCards(ids: [ID!]!): Int! }` (returns number of rows deleted).
- Extend the existing `NewCardInput` (`schema/schema.graphql:108-118`) with optional FSRS fields: `due, stability, difficulty, elapsedDays, scheduledDays, reps, lapses, state, lastReview` — all nullable. Existing required fields (`cardgroupId`, `front`, `back`) unchanged.
- Validation: all overrides `null` → use `domain.NewFSRSStateDefault()`. Mixed null + value is **rejected** (all-or-nothing) to avoid invalid combinations.

Backend:
- `repository/card.go`: `DeleteByIDsTx(ctx, tx, ids []string) (int64, error)`. Owner-scoped (the SQL uses a subselect that joins `cardgroups` to enforce ownership at the DB layer; usecase still does an explicit owner check first).
- `usecase/card.go`: `BulkDelete(ctx, ids)` with batched owner check (one query: `SELECT id, cardgroup_id FROM cards WHERE id IN ?`, then verify all cardgroup ownerships in one DataLoader-style call).
- `domain/fsrs_state.go`: add `NewFSRSStateFromInput(input)` factory that validates "all or nothing" and returns the value object.
- `usecase/card.go::Create`: branch on whether input has FSRS fields; if yes, use the override; if no, default.

Frontend:
- `frontend/src/app/cardgroups/[id]/cards/page.tsx`: add per-row checkbox + "Delete N selected" action button + confirmation dialog.
- Apollo cache update: on `deleteCards` success, evict the deleted IDs from the Connection.

**Out of scope**:
- Soft delete (we hard-delete, matching the old impl).
- "Undo" UX.
- FSRS override is **not** exposed in the UI (admin-only via dictionary import in PR-07).

**Anchor files**:
- `schema/schema.graphql:108-118` (`NewCardInput` to extend).
- `schema/schema.graphql:142-148` (Mutation block to extend).
- `backend/internal/repository/card.go` (existing card repo).
- `backend/internal/usecase/card.go:175` (`authorizeCardgroup`).
- `backend/internal/domain/fsrs_state.go` (FSRS factory location).
- `frontend/src/app/cardgroups/[id]/cards/page.tsx` (UI to extend with selection state).

**DB impact**:
- No schema change (cards already has all FSRS columns). `DELETE WHERE id = ANY(?)` is the SQL.

**Tests** (additional):
- `backend/internal/usecase/card_bulk_delete_test.go`: delete own + foreign card → only own deleted, returns count = own.
- `backend/internal/usecase/card_create_with_fsrs_test.go`: defaults + full override + partial override (should reject).
- `frontend/__tests__/cards-bulk-delete.test.tsx`.

**Risks**:
- "Mixed null + value" rejection means a single typo in dictionary import would reject the whole batch. Document clearly. PR-07 should not pass partial values.
- `len(ids) == 0` short-circuit in the repo to avoid `WHERE id IN ()` GORM full-scan.

**Verification**:
- New tests pass.
- Dev: select 3 cards, delete, confirm 3 deleted from list.

---

### PR-05 — Adaptive learning (UserPerformanceService + UI)

**Goal**: Replace the `performanceMode: 0` placeholder with a real computation. Surface the mode in the swipe UI so the learner sees "next batch tuned for your current performance".

**Why now**: Card pagination + FSRS override are stable. The `SwipeResponse.performanceMode` slot already exists — this PR fills it.

**In scope**:

Domain layer:
- New `backend/internal/domain/service/user_performance.go` with:
  - `type PerformanceMetrics struct { SuccessRate, AvgDifficulty, RetentionRate float64; StudyStreak int; LapseRate float64; ReviewCount int }`.
  - `func ComputeMetrics(swipes []domain.SwipeRecord, now time.Time) PerformanceMetrics`.
  - `func ModeFromMetrics(m PerformanceMetrics) int` returning 0–4 (Difficult..InWhile) per legacy thresholds: 0.60 / 0.75 / 0.85 / 0.95 with `AvgDifficulty` adjustments at ±0.7 / ±0.3 boundaries.
  - Constants: `ModeDifficult=0, ModeDefault=1, ModeGood=2, ModeEasy=3, ModeInWhile=4`.
  - `ReviewCount < 20` → `ModeDefault` (legacy guard).

Repository:
- `repository/swipe_record.go`: `ListRecentByUser(ctx, userID, limit) ([]*domain.SwipeRecord, error)` (e.g., last 100 swipes). Indexed by `(user_id, reviewed_at DESC)` (already exists).

Usecase:
- `usecase/swipe.go::HandleSwipe`: after the existing FSRS update, call `swipeRepo.ListRecentByUser`, run `service.ComputeMetrics`, attach to `SwipeOutput.PerformanceMode`.
- Optional: cache per-user metrics for one minute in-memory (lru) — but YAGNI; skip unless profiling reveals a hot path.

Schema:
- Extend `SwipeResponse` with `metrics: PerformanceMetrics!` (non-null) and a new `type PerformanceMetrics { successRate: Float!, avgDifficulty: Float!, retentionRate: Float!, studyStreak: Int!, lapseRate: Float!, reviewCount: Int! }`.
- Update the `performanceMode` doc comment to remove "Always 0 until UserPerformanceService is ported".

Frontend:
- `frontend/src/app/learn/[cardgroupId]/learn-client.tsx`: read `performanceMode` from response; show a small badge ("Mode: Easy", color-coded). Optionally show `successRate %`.
- Adjust next-batch size hint or copy text per mode (e.g., mode 0 → "Take it slow, 5 cards next" / mode 4 → "You're flying, 20 cards next"). Backend already returns nextCards; UI just adapts copy.

**Out of scope**:
- Server-side change to **batch size** based on mode (would require changing `defaultSwipeNextBatchSize` per request — propose for follow-up; not in this PR).
- Detailed analytics dashboard.

**Anchor files**:
- `schema/schema.graphql:158-163` (existing `SwipeResponse` to extend).
- `backend/internal/usecase/swipe.go:34-52` (struct to extend with PerformanceMode return).
- `backend/internal/repository/swipe_record.go` (add `ListRecentByUser`).
- `backend/internal/domain/service/` (new file location).
- `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (UI integration point).
- `docs/backend.md` (add a "UserPerformanceService" subsection under domain services; describe the threshold table and the `< 20 reviews → ModeDefault` guard).
- Old reference: `tmp/flamingo-armond-old/backend/graph/services/user_performance.go:1-257` (algorithm to translate, not copy).

**DB impact**:
- Index `idx_swipe_records_user_reviewed` on `(user_id, reviewed_at DESC)` already exists (`initial_schema.up.sql:175`).

**Tests** (additional):
- `backend/internal/domain/service/user_performance_test.go`: table-driven for each metric calculator + thresholds.
- `backend/internal/usecase/swipe_performance_test.go`: integration — < 20 swipes → mode 1, 60% success → mode 0, 95% success → mode 4.
- `frontend/__tests__/learn-mode-badge.test.tsx`.

**Risks**:
- Loading the last 100 swipes per swipe API call is fine (single indexed query) but increases the per-swipe cost. If pagination ever pushes 10k cards/group, revisit.
- Time-zone for `StudyStreak`: use server time (`time.Now()` UTC) and document.

**Verification**:
- Tests pass.
- Manual: swipe 20+ Easy ratings in a row, verify mode advances; misclick repeatedly, verify mode regresses.

**Implementation task order and progress**:

Tasks are ordered by dependency:

1. [x] Add the domain performance calculator and focused tests.
   - Dependency: legacy algorithm reference only.
   - Parallel group A: can run alongside tasks 2 and 3.
   - Blocks: swipe usecase integration.
   - Progress: completed locally on 2026-04-30.
2. [x] Add `SwipeRecordRepository.ListRecentByUser` and repository coverage.
   - Dependency: existing `swipe_records` schema and `(user_id, reviewed_at DESC)` index.
   - Parallel group A: can run alongside tasks 1 and 3.
   - Blocks: swipe usecase integration.
   - Progress: completed locally on 2026-04-30.
3. [x] Extend GraphQL schema with `PerformanceMetrics` and `SwipeResponse.metrics`.
   - Dependency: existing `SwipeResponse`.
   - Parallel group A: can run alongside tasks 1 and 2.
   - Blocks: gqlgen and frontend codegen.
   - Progress: completed locally on 2026-04-30.
4. [x] Regenerate gqlgen and integrate metrics/mode into `SwipeUsecase.HandleSwipe`.
   - Depends on: tasks 1, 2, and 3.
   - Blocks: frontend query/codegen and resolver mapping.
   - Progress: completed locally on 2026-04-30.
5. [x] Add backend usecase integration tests for performance mode.
   - Depends on: task 4.
   - Parallel group B: can run alongside tasks 6 and 7 after task 4.
   - Progress: completed locally on 2026-04-30.
6. [x] Update learn UI, frontend query, codegen output, and badge test.
   - Depends on: task 4 and generated GraphQL schema.
   - Parallel group B: can run alongside tasks 5 and 7 after task 4.
   - Progress: completed locally on 2026-04-30.
7. [x] Update backend docs for `UserPerformanceService`.
   - Depends on: task 1 behavior being fixed.
   - Parallel group B: can run alongside tasks 5 and 6 after task 4.
   - Progress: completed locally on 2026-04-30.
8. [x] Run verification commands and language-policy checks.
   - Depends on: tasks 5, 6, and 7.
   - Progress: completed locally on 2026-04-30.

---

### PR-06 — Dictionary parser (goyacc) + validateDictionary

**Goal**: Bring in the goyacc-driven dictionary text parser as a reusable Go package, exposed via a `validateDictionary` Query. The Mutation that actually inserts cards is in PR-07 — this PR is about parser + preview only.

**Why now**: Bulk delete + FSRS override unblock dictionary upserts cleanly. Parser is the heaviest non-feature dependency; isolating it in its own PR keeps the size manageable.

**In scope**:

Tooling:
- Add `goyacc` to `backend/tools.go` so `go tool goyacc -o ... grammar.y` works through the standard tool pipeline.
- Add a `Makefile` target `codegen-yacc` that re-runs goyacc on `backend/internal/textdic/grammar.y`.

Backend:
- New package `backend/internal/textdic/`:
  - `grammar.y` (yacc grammar, copied from `tmp/flamingo-armond-old/backend/pkg/textdic/grammar.y` and adapted).
  - `parser.go` (generated, committed for fewer surprises in CI; regenerated only via `make codegen-yacc`).
  - `service.go` exporting:
    ```go
    type ParsedWord struct { Front, Back string; Line int }
    type ValidationError struct { Line int; Message string }
    func Process(input string) (words []ParsedWord, errs []ValidationError, err error)
    ```

Schema:
- New types:
  ```graphql
  input ValidateDictionaryInput {
    """Base64-encoded dictionary text. Tab- or space-separated front/back per line."""
    payload: String!
  }
  type DictionaryValidationResult {
    valid: Boolean!
    parsedWords: [ParsedWord!]!
    errors: [DictionaryValidationError!]!
  }
  type ParsedWord { front: String!, back: String!, line: Int! }
  type DictionaryValidationError { line: Int!, message: String! }

  extend type Query {
    """Validate a base64-encoded dictionary payload without persisting. Admin-only."""
    validateDictionary(input: ValidateDictionaryInput!): DictionaryValidationResult!
  }
  ```

Resolver:
- `validateDictionary` resolver decodes base64, calls `textdic.Process`, returns the structured result.
- Reject non-admin via `auth.Service.IsAdmin(ctx, callerID)` → `gqlerr.NewForbidden(...)` (uses PR-01).

**Out of scope**:
- No `upsertDictionary` Mutation (PR-07).
- No frontend page (PR-07 is the admin form that consumes this).
- No streaming / chunked parsing — full payload at once is fine for ≤ 1MB inputs (document the limit; `MaxRequestBodySize` in Echo).

**Anchor files**:
- Old reference: `tmp/flamingo-armond-old/backend/pkg/textdic/grammar.y`.
- Old reference: `tmp/flamingo-armond-old/backend/pkg/textdic/text_dictionary_service.go`.
- `backend/internal/auth/role.go` (PR-01 helper to call).
- `backend/internal/gqlerr/errors.go` (`NewForbidden`).
- `schema/schema.graphql` (extend Query block).

**DB impact**:
- None.

**Tests** (additional):
- `backend/internal/textdic/service_test.go`: golden inputs (clean, with mixed tabs/spaces, with blank lines, with bad rows).
- `backend/internal/textdic/parser_test.go`: regression for grammar (does `make codegen-yacc` produce a stable parser).

**Risks**:
- `goyacc` generated code can diverge across Go versions; **commit the generated parser** to avoid CI nondeterminism. CI verifies with `git diff --exit-code` after running `make codegen-yacc` (gate).
- Base64 padding/URL-safe variant — be explicit about which alphabet we accept (standard, with padding).

**Verification**:
- `make codegen-yacc && git diff --exit-code` is clean.
- Tests pass.
- GraphiQL: `validateDictionary` rejects non-admin, returns parsed words for admin.

---

### PR-06.2 — Follow-up triage of deferred review items (issue #65)

See `.claude/plans/improve_pr65_progress.md` for scope, parallel waves, and per-cluster status. Branch: `feature/improve_pr6_2`.

---

### PR-07 — upsertDictionary + admin import page

**Goal**: Persist a parsed dictionary into a target cardgroup and ship the admin UI for it.

**In scope**:

Schema:
- New input + Mutation:
  ```graphql
  input UpsertDictionaryInput {
    cardgroupId: ID!
    payload: String!  # base64-encoded
  }
  type UpsertDictionaryPayload {
    inserted: Int!
    updated: Int!
    errors: [DictionaryValidationError!]!
  }
  extend type Mutation {
    upsertDictionary(input: UpsertDictionaryInput!): UpsertDictionaryPayload!
  }
  ```

Backend:
- `usecase/dictionary.go`: parse payload, validate cardgroup ownership (admin can target any), upsert cards via `repository/card.go::UpsertManyTx`. Conflict key: `(cardgroup_id, front)` (composite uniqueness; needs a partial index — see DB impact).
- `repository/card.go`: `UpsertManyTx(ctx, tx, cards []domain.Card) (inserted, updated int64, error)`. Use `ON CONFLICT (cardgroup_id, front) DO UPDATE SET back = EXCLUDED.back, updated_at = now() RETURNING xmax = 0` to count inserts vs updates.
- Each new card uses `NewFSRSStateDefault()` (override input from PR-04 not exposed here — kept simple).

Migration:
- New `<ts>_add_cards_upsert_index.up.sql` adds `CREATE UNIQUE INDEX IF NOT EXISTS uq_cards_cardgroup_front ON public.cards (cardgroup_id, front);`. Required by `ON CONFLICT`.
- Down migration drops the index.

Frontend:
- New page `frontend/src/app/admin/dictionary/page.tsx`:
  - Server-side admin gate (call `me` query → check role; redirect non-admin to `/`).
  - Cardgroup selector (only admin-visible since admin can target any).
  - `<textarea>` for paste; on Validate click → call `validateDictionary` → render preview table with errors highlighted.
  - On Import click → call `upsertDictionary` → toast "X inserted, Y updated, Z errors".

**Out of scope**:
- File upload (paste-only is fine; matches the old impl).
- Cardgroup creation in the import flow (cardgroup must pre-exist; admin uses the regular create flow).

**Anchor files**:
- `schema/schema.graphql` (extend Mutation block; see `:142`).
- `backend/internal/repository/card.go` (extend with upsert).
- `backend/internal/usecase/` (new `dictionary.go`).
- Old reference: `tmp/flamingo-armond-old/backend/pkg/usecases/dictionary_manager/dictionary_manager_usecase.go:1-77`.
- Old reference: `tmp/flamingo-armond-old/frontend/src/app/(admin)/admin/words/manage/words-register.tsx:1-315` (UI to translate, not copy verbatim).

**DB impact**:
- Unique index `(cardgroup_id, front)`. Verify no existing duplicates before creating.

**Tests** (additional):
- `backend/internal/usecase/dictionary_test.go`: 100-card payload upsert, 50 inserts + 50 updates, 0 errors.
- `frontend/__tests__/admin-dictionary.test.tsx`: validate flow, error preview, import success path.

**Risks**:
- Existing duplicate `(cardgroup_id, front)` rows would block index creation. PR includes a check migration (`SELECT cardgroup_id, front, COUNT(*) FROM cards GROUP BY 1,2 HAVING COUNT(*) > 1`) and fails fast with a clear error if found. (Currently no UI creates duplicates, so this should be empty.)
- Card limit per request: cap at 5,000 rows per upsert to keep transaction time bounded; reject larger with a clear error.

**Verification**:
- Tests pass.
- Dev: paste a 200-row CSV-like payload, see correct insert/update counts.

---

### PR-08 — Admin: user list / edit / role assignment

**Goal**: Admin can list users, edit display_name / bio, and assign / revoke roles.

**In scope**:

Schema:
- **First introduction of `type Role` in the GraphQL schema** (the Go `domain.Role` already exists; this surfaces it):
  ```graphql
  type Role { id: ID!, name: String! }
  ```
- Connection types: `UserConnection`, `UserEdge`.
- Queries:
  ```graphql
  extend type Query {
    """Admin-only. Paginated user list."""
    users(first: Int, after: ID, last: Int, before: ID, search: String): UserConnection!
    """Admin-only. Single user with roles populated."""
    adminUser(id: ID!): User
  }
  extend type User { roles: [Role!]! }
  ```
- Mutations:
  ```graphql
  input AdminUpdateUserInput { displayName: String, bio: String }
  extend type Mutation {
    adminUpdateUser(id: ID!, input: AdminUpdateUserInput!): User!
    assignRole(userId: ID!, roleId: ID!): User!
    revokeRole(userId: ID!, roleId: ID!): User!
  }
  ```

Backend:
- `usecase/admin_user.go`: list/search users + edit + assign/revoke. All gated by `auth.Service.IsAdmin` (PR-01).
- `repository/user.go`: add `ListPage(...)`, `Search(...)`. (User repo currently has only `me`-style methods.)
- `repository/role.go`: extend `RoleRepository` with `AssignToUser(ctx, userID, roleID)`, `RevokeFromUser(ctx, userID, roleID)`, `ListByUser(ctx, userID) ([]*domain.Role, error)`. (Existing methods `FindByName`, `FindByIDs` stay unchanged.)
- DataLoader: new `loader.RoleByUserID` (note: distinct from the existing `loader.Role` which loads roles by role ID). Use this for the `User.roles` resolver to avoid N+1.

Frontend:
- `frontend/src/app/admin/users/page.tsx`: list with search; admin-only (server-side check).
- `frontend/src/app/admin/users/[id]/edit/page.tsx`: form with role multi-select (checkbox per existing role).
- Apollo cache: optimistic update for assign/revoke; refetch user on success.

**Out of scope**:
- User deletion (would need cascade strategy across cardgroups/cards/swipe_records — deferred).
- Inviting new users (handled by Supabase Auth; admin uses Supabase dashboard for that, not this UI).
- Password reset (Supabase Auth handles; out of scope).
- Sorting columns; default `created_at DESC`.

**Anchor files**:
- `schema/schema.graphql:6-21` (existing User type to extend with `roles`).
- `backend/internal/repository/user.go` (add list/search).
- `backend/internal/loader/loader.go:15` (add `RoleByUserID`).
- `backend/internal/auth/role.go` (PR-01 helper).
- Old reference: `tmp/flamingo-armond-old/frontend/src/app/(admin)/admin/users/page.tsx`.
- Old reference: `tmp/flamingo-armond-old/frontend/src/app/(admin)/admin/users/[id]/edit/page.tsx`.

**DB impact**:
- None (queries against existing tables).

**Tests** (additional):
- `backend/internal/usecase/admin_user_test.go`: non-admin → forbidden; admin → list/edit/assign works.
- `frontend/__tests__/admin-users.test.tsx`: list rendering, role checkbox toggles call assign/revoke.

**Risks**:
- Admin removing their own admin role — guard against that on the server (`if uid == callerID && roleName == "admin"` → reject with `FORBIDDEN`).
- Pagination cursor is the user UUID; same pattern as PR-03.

**Verification**:
- Tests pass.
- Dev: as admin, view user list, edit a row, toggle admin role on a second user, see changes.

---

### PR-09 — Admin: role CRUD + admin layout/nav

**Goal**: Admin can manage role definitions (CRUD) and the admin section has a unified layout (sidebar + header) for the user/role/dictionary pages.

**In scope**:

Schema:
- `extend type Query { roles: [Role!]!; role(id: ID!): Role }` (`Role` type was introduced in PR-08).
- `extend type Mutation { createRole(name: String!): Role!; updateRole(id: ID!, name: String!): Role!; deleteRole(id: ID!): Boolean! }`.
- Reject deletion of role `admin` at the usecase layer (immutable system role).

Backend:
- `usecase/admin_role.go`: CRUD + system-role guard. Admin-gated.
- `repository/role.go` (PR-01): extend with `Update`, `Delete`.

Frontend:
- New layout `frontend/src/app/admin/layout.tsx`:
  - Server-side admin gate (single source of truth — used by all admin pages, simplifies guards).
  - Sidebar with links to Users, Roles, Dictionary.
  - Header with sign-out + back-to-app link.
- `frontend/src/app/admin/roles/page.tsx`: list with inline edit + add row.

**Out of scope**:
- Role permissions matrix (single binary admin/general is enough; "permissions per role" is a follow-up if ever needed).
- Renaming the `admin` role in flight (forbidden; system role).

**Anchor files**:
- `frontend/src/app/cardgroups/_layout.tsx` (or root `layout.tsx`) — pattern for layouts.
- Old reference: `tmp/flamingo-armond-old/frontend/src/components/admin/AdminDrawer.tsx:1-140`.
- `backend/internal/repository/role.go` (PR-01).

**DB impact**:
- None.

**Tests** (additional):
- `backend/internal/usecase/admin_role_test.go`: create + rename; cannot delete admin.
- `frontend/__tests__/admin-roles.test.tsx`.

**Risks**:
- Casing: role names are stored lowercase; UI displays Title-cased; resolvers must normalize.
- The admin layout becomes the **single gate** for admin pages; all admin children rely on it. Make sure server-side gate runs before any client component bootstraps.

**Verification**:
- Tests pass.
- Dev: admin section navigates between Users / Roles / Dictionary without re-checking admin status on each page.

---

### PR-10 — Vercel deploy CI job

**Goal**: Add a `deploy` job to `.github/workflows/frontend.yml` that runs only on `push` to `main` after the existing `lint-test-build` job, using Vercel CLI.

**Why split out**: Independent of all other PRs (CI/infra). Smallest PR, can land in parallel with feature PRs once we have a Vercel project linked.

**In scope**:
- Add `deploy` job to `frontend.yml` with:
  - `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`.
  - `needs: lint-test-build`.
  - Steps: `vercel pull --yes --environment=production --token=$VERCEL_TOKEN` → `vercel build --prod --token=$VERCEL_TOKEN` → `vercel deploy --prebuilt --prod --token=$VERCEL_TOKEN`.
- New repo secrets: `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` (documented in `docs/ci.md`).
- Update `docs/deployment.md` with the Vercel deploy section.

**Out of scope**:
- Removing Vercel's automatic Git integration (we keep it; CI is a redundant deploy path that gates on `lint-test-build`). Document that both exist; mark CI deploy as authoritative going forward and remove the Git integration in a follow-up.

**Anchor files**:
- `.github/workflows/frontend.yml` (add `deploy` job after `lint-test-build`).
- `docs/ci.md` (add deploy job decision).
- `docs/deployment.md` (Vercel section).

**Risks**:
- Two deploy paths (Vercel Git auto-deploy + this CI deploy) can race. Preferred resolution: keep CI deploy, disable Vercel Git integration in a follow-up doc-only PR.

**Verification**:
- Open a test PR that does not trigger deploy (PR event).
- Merge to `main`, observe `deploy` job runs and produces a production URL.

**Progress (issue #49, branch `feature/improve_pr10`)**:

Cluster grouping for parallel execution:

| Cluster | File                              | Owner agent (model) | Status    | Notes |
|---------|-----------------------------------|---------------------|-----------|-------|
| A       | `.github/workflows/frontend.yml`  | sub-A (Sonnet 4.6)  | completed | `deploy` job added at lines 79-119 with VERCEL_TOKEN guard |
| B       | `docs/ci.md`                      | sub-B (Sonnet 4.6)  | completed | New `### Frontend deploy job` subsection (lines 138-160) |
| C       | `docs/deployment.md`              | sub-C (Sonnet 4.6)  | completed | New `### Vercel CI deploy` subsection (lines 244-301) |
| C-fix   | `docs/deployment.md`              | fix agent (Haiku)   | completed | `gh secret set --body` → `printf \| gh secret set` (stdin pattern, per § "Two security patterns") |
| Commit  | (all clusters)                    | commit agent (Haiku)| completed | `34e4241 ci(frontend): add Vercel deploy job` (single commit; plan file stays uncommitted because `.claude/plans/` is gitignored) |

PR review loop (Step 2):

| Iter | Findings (Critical/Important)                                                                                                              | Resolution                                                                                                                              | Commit |
|------|--------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------|--------|
| 1    | C: missing pnpm install / org-project guards / docs --token mismatch. I: --token in argv / vercel@latest / verbatim if: / "identically" / build-time env vars | env-level token, 3-secret guard, pnpm install + cache, vercel@52 pin, doc accuracy fixes                                                | `7475a92 ci(frontend): harden Vercel deploy job after review` |
| 2    | C: invalid cross-ref / "today" framing in ci.md. I: workflow concurrency / "same artifact" claim / cache rationale                         | concurrency override on deploy job, doc cross-ref tighten, transitional framing, conditional artifact equivalence                       | `14416d4 ci(frontend): protect deploy from mid-flight cancellation` |
| 3    | C: `frontend/.tool-versions` does not exist. I: concurrency exception undocumented                                                         | `.tool-versions` (repo root), new bullet in § "Workflow scope and concurrency"                                                          | `10e86a7 docs(ci): correct .tool-versions path and document deploy concurrency` |
| 4    | none — comment-analyzer + code-reviewer both report clean                                                                                  | —                                                                                                                                       | — |

**Deferred (out of scope for this PR)**:
- Vercel CLI exact-pin (`vercel@52.0.3`) — repo's `@vN` convention favors auto-follow patch/minor for security fixes
- `pnpm store path` empty-string swallow at lines 51 + 113 — pre-existing pattern across both jobs; warrants its own follow-up

**Final commit list (6 commits on `feature/improve_pr10`)**:
1. `34e4241 ci(frontend): add Vercel deploy job` — Wave 1 implementation
2. `7475a92 ci(frontend): harden Vercel deploy job after review` — review iteration 1 fixes
3. `14416d4 ci(frontend): protect deploy from mid-flight cancellation` — review iteration 2 fixes
4. `10e86a7 docs(ci): correct .tool-versions path and document deploy concurrency` — review iteration 3 fixes
5. `641f257 docs(ci): split pnpm-install paragraph for readability` — code-simplifier output
6. `d117a2f docs: generalize CI patterns from Vercel deploy work` — Step 5 promotions

**Step 4 (tests) results**: actionlint clean; CJK + PR-order grep clean; backend `go vet` + `go build` + `go test -short` all pass; frontend `pnpm typecheck` + `pnpm lint` clean; Vitest 27 files / 207 tests all pass.

**Step 5 (learnings) — three promotions to general principles**:
- `docs/ci.md` § "Workflow scope and concurrency": added "General rule" sentence on `cancel-in-progress` policy (compute-only jobs vs external-state jobs).
- `docs/ci.md` § "GitHub Actions versioning": added "In-band npm tool installs follow the same policy" paragraph (Vercel CLI `vercel@52` as example).
- `docs/deployment.md` § "Two security patterns" → renamed "Three security patterns"; added third bullet on CLI tokens via env (not `--token` flag); cross-references updated.

Skipped (already adequately covered in PR's docs): Vercel CLI 3-step flow, `pnpm install` for `vercel build`, build-time env vars trap, cache key sharing rationale, Node version mismatch risk.

Dependency order (Wave 1 = A/B/C parallel; Wave 2 = commit; Wave 3 = review/simplify/test/learnings).

Then loop:
- Step 2: `/pr-review-toolkit:review-pr` until no critical/important findings remain in scope
- Step 3: `code-simplifier` on changed + related files
- Step 4: unit + integration tests, autonomous remediation
- Step 5: extract why/what learnings into `docs/`

---

### PR-11 — Vitest page tests revival

**Goal**: Add Vitest + Testing Library tests for the user-facing pages (register, cardgroups list, cardgroup detail, cards list) and the admin pages (users, roles, dictionary).

**Why now**: All features they cover have shipped (PR-03 through PR-09). Tests are not blocking earlier PRs (those carry their own narrow tests), but a broader suite catches regressions across PRs.

**In scope**:
- `frontend/__tests__/cardgroups-list.test.tsx`
- `frontend/__tests__/cardgroups-detail.test.tsx`
- `frontend/__tests__/cards-list.test.tsx` — broad list rendering (selection, empty state, error state). Distinct from `cards-pagination.test.tsx` shipped in PR-03 (which only exercises Intersection Observer + `fetchMore`).
- `frontend/__tests__/register.test.tsx` (registration / sign-up flow).
- `frontend/__tests__/admin-users.test.tsx` (broad coverage; PR-08 ships a narrower test focused on assign/revoke).
- `frontend/__tests__/admin-roles.test.tsx` (broad coverage; PR-09 ships role-CRUD-only tests).
- `frontend/__tests__/admin-dictionary.test.tsx` (broad coverage; PR-07 ships an upsert-flow-only test).
- New shared fixture file `frontend/__tests__/fixtures/users.ts` (admin / general user).
- Update `frontend/vitest.config.ts` if needed (paths, jsdom, setup).

**Naming convention** for tests across PRs:
- Each feature PR ships a **narrow** test (one specific flow it implements; lives in `frontend/__tests__/<feature>-<flow>.test.tsx`, e.g., `cards-pagination.test.tsx`).
- PR-11 ships **broad page-level** tests (one per page; `frontend/__tests__/<page>.test.tsx`).
- Avoid duplication by naming consistently from the start.

**Out of scope**:
- E2E (PR-12).
- Backend tests (each feature PR added its own).
- Coverage thresholds (use Codecov reports for visibility; do not gate CI yet).

**Anchor files**:
- `frontend/src/app/cardgroups/_components/profile-form.test.tsx` (existing pattern).
- Old reference: `tmp/flamingo-armond-old/frontend/__tests__/admin-users.test.tsx` (1268 lines — translate the cases, do not copy: old tests use REST helpers + AuthContext, new tests use Apollo `MockedProvider` + Supabase client mocks).

**DB impact**:
- None.

**Risks**:
- Mocking Supabase client requires an in-memory implementation of `getUser`; provide a tiny test util.
- Apollo `MockedProvider` for paginated queries: each `fetchMore` call needs a separate mock; document the pattern in the test util.

**Verification**:
- `pnpm --filter frontend test` passes locally and in CI.
- Coverage report shows ≥ 60% for the relevant page files.

**Note on LoC accounting**: tests are not counted in the 800-line cap. This PR is allowed to be larger as long as it is purely tests + fixtures.

---

### PR-12 — Playwright E2E + workflow

**Goal**: Introduce Playwright with two end-to-end flows — admin import + learner swipe — and a dedicated CI workflow.

**In scope**:
- `frontend/playwright.config.ts` configured to run against a local dev server (`pnpm --filter frontend dev` via `webServer` block) using a Supabase test project (or local `supabase start` if available in CI).
- Tests:
  - `frontend/e2e/admin-import.spec.ts`: log in as seeded admin → go to `/admin/dictionary` → paste payload → validate → import → verify cards appear.
  - `frontend/e2e/learn-flow.spec.ts`: log in as seeded learner → go to `/learn/<cardgroupId>` → swipe Easy three times → verify next batch arrives and mode badge updates.
- New workflow `.github/workflows/e2e.yml`:
  - Trigger: `pull_request` paths (`frontend/**`, `schema/**`) + `push` to `main` + nightly cron.
  - Job runs `supabase start` (or uses test Supabase) → seeds DB (`supabase db reset --seed`) → starts backend (`go run ./cmd/server` in background) → starts frontend → runs Playwright.
  - Upload Playwright traces on failure.

**Out of scope**:
- Visual regression (deferred).
- Cross-browser (Chromium only initially).

**Anchor files**:
- `.github/workflows/frontend.yml` (sibling reference for env / mise / pnpm setup).
- `frontend/package.json` (add `@playwright/test` dependency + `test:e2e` script here).
- `supabase/seed.sql` (PR-13 — but PR-12 ships an inline temp seed in the workflow if it lands first).
- Old reference: `tmp/flamingo-armond-old/playwright-config.json` does not match Playwright's canonical config name; do not copy. Use the canonical `frontend/playwright.config.ts` per Playwright docs.

**Dependencies**:
- Depends on PR-09 (admin layout/nav must exist for the admin import flow to navigate).
- Depends on PR-10 (Vercel deploy CI is unrelated, but consistent CI patterns help).
- Light dependency on PR-13 (seed); see § "Risks" below for mitigation if PR-12 lands first.

**DB impact**:
- None (tests use a disposable DB via Supabase CLI).

**Risks**:
- `supabase start` in CI takes ~30 s; consider caching the Supabase Docker images.
- Test isolation: each spec must reset DB or use unique test data. Prefer reset-per-spec via `supabase db reset` for simplicity.
- If PR-13 has not landed yet, PR-12 ships an inline seed inside the workflow (a small temp `seed.sql` written by the workflow). Once PR-13 lands, switch to that file.

**Verification**:
- Both specs green locally with `pnpm --filter frontend test:e2e`.
- New workflow green on PR.

**Note on LoC accounting**: same as PR-11; tests not counted toward 800.

---

### PR-13 — Dev seed (`supabase/seed.sql`)

**Goal**: One Supabase seed file that, on `supabase db reset`, gives a working local environment with an admin, a regular user, two cardgroups, and ~20 cards.

**In scope**:
- New `supabase/seed.sql` with:
  - Two `auth.users` rows (using `INSERT INTO auth.users` directly with hashed passwords or relying on the `handle_new_user` trigger if Supabase CLI permits — verify the canonical Supabase pattern).
  - Two `public.users` rows (created by the trigger).
  - One `INSERT INTO public.user_roles` linking admin user to admin role.
  - Two `public.cardgroups` rows (one per user).
  - ~20 `public.cards` rows split across the two cardgroups, with default FSRS state.
  - Comment documenting that this is **dev-only** and not run in production.
- Update `docs/dev-setup.md` with "Run `supabase db reset` to load the seed".
- Smoke check: log in locally as admin → admin dashboard renders.

**Out of scope**:
- Prod seed (out of scope per Q8).
- Bcrypt/Argon hash generation in SQL — use Supabase's `crypt` extension or a fixed pre-hashed dev password (document the dev password in `docs/dev-setup.md`, never in production).

**Anchor files**:
- `supabase/config.toml` (existing; defines the local Supabase instance).
- `backend/internal/database/migrations/20260430080000_initial_schema.up.sql` (the schema this seeds against).
- Old reference: `tmp/flamingo-armond-old/backend/db/seeds/dev/` (concept reference; old uses goose seeds, new uses Supabase seed.sql).

**DB impact**:
- Idempotent inserts: `ON CONFLICT DO NOTHING` for users/cardgroups/cards.

**Risks**:
- Supabase Auth's hash format may change; **lock the seeded password to a documented constant** and refresh the seed if Supabase Auth ever rotates the format.
- If `auth.users` cannot be inserted directly via SQL (Supabase may restrict it), use Supabase Admin REST during `supabase db reset` hooks. Verify which path works locally first.

**Verification**:
- `supabase db reset` succeeds.
- `pnpm --filter frontend dev` and login as the seeded admin reach the admin dashboard.

---

## 8. Migration ordering (DB) summary

```
20260430080000_initial_schema                (existing)
20260501000000_add_ping_records              (existing)
<ts>_add_rbac_helpers                        (PR-01)  -- defines is_admin()
<ts>_add_rls_policies                        (PR-02)  -- requires is_admin() from PR-01
<ts>_add_cards_upsert_index                  (PR-07)  -- requires no duplicate (cardgroup_id, front)
```

Each new `<ts>` is `YYYYMMDDHHMMSS` UTC at the moment the PR opens. Filenames are immutable after merge; do not rename if a later branch renumbers.

## 9. Schema evolution summary

```
+ enum CardOrderBy                                       (PR-03)
+ enum SortOrder                                         (PR-03)
+ type CardConnection / CardEdge / PageInfo              (PR-03)
+ Query.cardsByCardgroupConnection                       (PR-03)
@deprecated cardsByCardgroup                             (PR-03)
+ Mutation.deleteCards                                   (PR-04)
~ NewCardInput (FSRS optional fields added)              (PR-04)
+ type PerformanceMetrics                                (PR-05)
~ SwipeResponse.metrics (added; performanceMode now real) (PR-05)
+ Query.validateDictionary + supporting types            (PR-06)
+ Mutation.upsertDictionary                              (PR-07)
+ Query.users / adminUser                                (PR-08)
+ User.roles                                             (PR-08)
+ Mutation.adminUpdateUser / assignRole / revokeRole     (PR-08)
+ input AdminUpdateUserInput                             (PR-08)
+ type Role                                              (defined in PR-08)
+ Query.roles / role                                     (PR-09)
+ Mutation.createRole / updateRole / deleteRole          (PR-09)
```

## 10. Pitfalls watchlist (carry across PRs)

1. **gqlgen regeneration**: any schema change requires `go tool gqlgen generate`. CI will catch missing regen but the failure mode is "resolver stub mismatch" — not always obvious.
2. **`schema.resolvers.go` managed block**: helpers must live outside the managed block or they get nuked. The standard convention in this repo is `resolver/helpers.go` (already exists).
3. **Echo v5 context**: signatures changed from v4. Anything pulled from old code referencing `echo.Context` (interface) becomes `*echo.Context` (pointer to struct). Ports must convert.
4. **DataLoader vs Tx**: do not call DataLoaders inside a Tx — they use a separate connection. Use `xxxTx` repo methods inside Tx.
5. **GORM `IN ()` empty-slice**: always early-return on `len(ids) == 0`.
6. **`fmt.Errorf("%w")` ban**: use `eris.Wrap`. CI greps.
7. **Resolver field name collisions**: `Resolver.Cardgroup` (field) vs `Cardgroup()` (method on type Cardgroup). Use `XxxUC` suffix (`CardgroupUC`).
8. **`gorm.Model` embedding**: not used in this repo. Always declare columns + triggers explicitly.
9. **English-only verification**: run the two grep commands in `.claude/rules/language-policy.md` before each push.
10. **Supabase JWT roles**: Q1 = B means **JWT does not carry roles**; admin checks always go through DB. Do not be tempted to read `app_metadata.role` from the JWT in this iteration — it is not populated.
11. **CI path filters**: a PR touching only `schema/**` triggers `frontend.yml` (which depends on schema for codegen) but not `backend.yml`. If a schema change requires backend changes, mention `backend/**` paths or add the file explicitly.
12. **`docker-compose.yml`**: do not introduce. PR-12 (Playwright) uses Playwright's own `webServer` config, not Compose.

## 11. Issue template (use for each PR's GitHub Issue)

```markdown
# <PR title from § 6>

## Goal
<single sentence>

## In scope
- ...

## Out of scope
- ...

## Anchor files (existing seams)
- path:line — note

## DB impact
- migration filename or "none"

## Schema impact
- additions / deprecations

## Tests to add
- backend: ...
- frontend: ...
- e2e: ...

## Risks
- ...

## Verification
- [ ] `go test -race ./...` passes
- [ ] `pnpm --filter frontend lint && pnpm --filter frontend typecheck && pnpm --filter frontend build && pnpm --filter frontend test` passes
- [ ] Manual: <feature-specific check>
- [ ] Language-policy grep clean
- [ ] LoC of production code ≤ 800 (excludes tests, generated, vendored)

## Depends on
- PR-XX (link)
```

## 12. Open questions / known unknowns

- **PR-13 prerequisite**: confirm whether `auth.users` can be inserted via `supabase/seed.sql` directly, or whether Supabase CLI requires using an Admin API hook. Verify before opening PR-13.
- **PR-12 in CI**: Supabase CLI Docker startup time may push CI past 5 minutes; if so, consider running E2E only on nightly + post-merge instead of every PR. Decide once we measure.
- **PR-08 user search**: search uses `display_name ILIKE` — fine for hundreds of users; if user count grows, switch to `pg_trgm` index. Out of scope for now.
- **`cardsByCardgroup` deprecation removal**: deprecated in PR-03. Removal condition: after PR-11 lands, run `rg "cardsByCardgroup" frontend/src` and confirm zero matches outside `queries.ts` deprecation notes; then open a follow-up PR to delete the field, the resolver, and the deprecated query document. Tracked as a single tiny PR (≤50 lines) in a future cleanup wave.
