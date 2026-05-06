# Issue #102: Migrate `/admin/users` to shadcn DataTable

> Branch: `feature/admin-users-datatable-102`
> GitHub: https://github.com/yasuflatland-lf/flamingo-armond/issues/102

## Goals (from issue)
1. Backend: Add `User.lastActive: Time` field, populated at session-refresh
2. Backend: Add `roleId: ID` filter to admin `users(...)` connection
3. Frontend: Migrate `/admin/users` from `<ul>` + infinite scroll → shadcn `<Table>` + discrete pagination
4. 4 columns: Avatar+Name / Roles / LastActive / Actions
5. Server-side faceted role filter, 300ms search debounce, dropdown row actions

## Codebase findings (from research phase)

**Backend**
- `schema/schema.graphql:380-389` — `extend type User { roles, lastViewedCardgroup }` — add `lastActive` here.
- `schema/schema.graphql:406-408` — admin `users(first, after, last, before, search)` — add `roleId: ID`.
- `Time` scalar already declared at `schema/schema.graphql:45-46`.
- `backend/internal/domain/user.go:7-19` — User domain (no LastActive).
- `backend/internal/repository/user.go` — `ListPage` handles search via ILIKE+LIKE-escape; needs `roleId` join with `user_roles`.
- `backend/internal/usecase/admin_user.go:160-249` — `List`; thread `roleId`.
- `backend/graph/resolver/schema.resolvers.go:364-383` — Users resolver.
- `backend/internal/auth/middleware.go:21-61` — JWT verification path; insert `last_active = NOW()` write after context attachment (line 57).
- `backend/gqlgen.yml` — `User.roles` and `User.lastViewedCardgroup` are field-resolver:true. `lastActive` can be auto-generated (mapped from domain field).
- Migration template: `backend/internal/database/migrations/YYYYMMDDHHMMSS_*.up.sql`, `BEGIN; ... COMMIT;`

**Frontend**
- `frontend/src/app/admin/users/AdminUsersClient.tsx` (280 lines) — `<ul>` + IntersectionObserver + 300ms debounce.
- `frontend/src/app/admin/users/queries.ts` — `ADMIN_USERS_PAGE_SIZE=20`, no `ADMIN_USERS_DEFAULT_VARS`.
- `frontend/src/app/admin/users/page.tsx` — auth gate only, no SSR seed.
- `frontend/src/components/layout/listing-page-shell.tsx` — has `toolbar` slot. Reuse.
- **`@tanstack/react-table` is NOT installed** → vendor it.
- **No shadcn `<Table>`, `DataTablePagination`, `DataTableFacetedFilter`** primitives exist → vendor.
- Existing tests: `frontend/__tests__/admin-users-list.test.tsx` (610 lines, 7 cases).
- `frontend/__tests__/utils/mock-apollo-paginated.ts` — `installApolloMockLeakSpy` + `assertNoLeaks`.

## Cluster A — Backend (mostly sequential; A1 unblocks everything)

| ID | Subject | Files | Model | Depends |
|----|---------|-------|-------|---------|
| A1 | Schema: add `lastActive` + `roleId` arg | `schema/schema.graphql` | sonnet | — |
| A2 | DB migration: `users.last_active TIMESTAMPTZ NULL` + index | `backend/internal/database/migrations/2026...up.sql` (+ `.down.sql`) | sonnet | — |
| A3 | Domain + repo: add `LastActive`, `ListPage` joins `user_roles` for `roleId` filter | `backend/internal/domain/user.go`, `backend/internal/repository/user.go` | sonnet | A2 (col exists) |
| A4 | Usecase: thread `roleId` through `List` | `backend/internal/usecase/admin_user.go` (+ interface) | sonnet | A3 |
| A5 | Resolver: gqlgen regen, wire `roleId`, expose `User.LastActive` | `backend/gqlgen.yml`?, `backend/graph/resolver/schema.resolvers.go` | sonnet | A1, A4 |
| A6 | Session-refresh write: middleware writes `last_active = NOW()` async | `backend/internal/auth/middleware.go`, `backend/internal/repository/user.go` (`TouchLastActive`) | opus | A2, A3 |
| A7 | Backend tests: usecase (roleId + empty + nil), repo (last_active write), middleware (write boundary) | `backend/internal/usecase/admin_user_test.go`, `backend/internal/repository/user_test.go`, `backend/internal/auth/middleware_test.go` | sonnet | A1–A6 |

## Cluster B — Frontend (depends on Cluster A)

| ID | Subject | Files | Model | Depends |
|----|---------|-------|-------|---------|
| B0 | Vendor `@tanstack/react-table` + shadcn `<Table>`, `<DataTablePagination>`, `<DataTableColumnHeader>`, `<DataTableFacetedFilter>`, `<Avatar>`, `<Badge>`, `<DropdownMenu>` | `frontend/package.json`, `frontend/src/components/ui/*.tsx` | sonnet | — |
| B1 | Update `queries.ts`: add `lastActive`, `roleId` arg, export `ADMIN_USERS_DEFAULT_VARS` | `frontend/src/app/admin/users/queries.ts` | sonnet | A1 |
| B2 | Run frontend codegen | `pnpm codegen` | haiku | B1 |
| B3 | `users-columns.tsx`: 4 columns + relative-time helper | `frontend/src/app/admin/users/users-columns.tsx` | sonnet | B0, B2 |
| B4 | `users-toolbar.tsx`: search input + role faceted filter | `frontend/src/app/admin/users/users-toolbar.tsx` | sonnet | B0, B2 |
| B5 | `users-table.tsx`: `useReactTable` + DataTablePagination shell | `frontend/src/app/admin/users/users-table.tsx` | sonnet | B0, B2 |
| B6 | Rewrite `AdminUsersClient.tsx` — orchestrate B3/B4/B5, discrete pagination via `fetchMore`, URL-state for `page` + `roleId` + `search` | `frontend/src/app/admin/users/AdminUsersClient.tsx` | opus | B3, B4, B5 |
| B7 | Update `frontend/__tests__/admin-users-list.test.tsx` for new shape (4 columns, role-filter combine, MockedProvider leak spy across pagination + role-filter, PII absence) | `frontend/__tests__/admin-users-list.test.tsx` | sonnet | B6 |

## Cluster C — Quality gates (after Clusters A+B)

| ID | Subject | Tool / Model |
|----|---------|--------------|
| C1 | `/pr-review-toolkit:review-pr` loop until no Critical/Important in scope | review-pr skill |
| C2 | code-simplifier pass on changed files | code-simplifier agent |
| C3 | Run unit + integration tests; fix failures autonomously | sonnet (escalate to opus on stuck) |
| C4 | Document Why+What learnings into `docs/` and `.claude/rules/` | sonnet |

## Parallelism plan

- **Wave 1 (parallel)**: A1, A2, B0
- **Wave 2 (parallel after A1+A2)**: A3, A6 (uses repo helper added by A3 — actually A6 depends on A3 for `TouchLastActive` repo helper; serialize) — revise: **Wave 2 = A3 alone**
- **Wave 2.5**: A6 in parallel with A4 (A4 depends on A3, A6 depends on A3 — both can proceed)
- **Wave 3 (after A4)**: A5
- **Wave 4 (after A5)**: A7
- **Wave 5 (after A1)**: B1
- **Wave 6 (after B1)**: B2
- **Wave 7 (after B2 + B0)**: B3, B4, B5 in parallel
- **Wave 8 (after B3+B4+B5)**: B6
- **Wave 9 (after B6)**: B7

## Commit plan (handed to dedicated commit agent)

Each commit ~1 logical unit (no interleaving). Suggested boundaries:
1. `feat(schema): add User.lastActive and roleId filter to admin users query` (A1)
2. `feat(backend): migrate users.last_active column` (A2)
3. `feat(backend): plumb lastActive + roleId filter through repo+usecase` (A3, A4)
4. `feat(backend): expose lastActive resolver and roleId arg` (A5)
5. `feat(backend): write last_active on every authenticated request` (A6)
6. `test(backend): cover lastActive write + roleId filter` (A7)
7. `chore(frontend): vendor @tanstack/react-table + shadcn DataTable primitives` (B0)
8. `feat(frontend): wire lastActive + roleId into admin users queries` (B1, B2)
9. `feat(frontend): add admin users DataTable columns/toolbar/table shell` (B3, B4, B5)
10. `feat(frontend): migrate AdminUsersClient to DataTable + discrete pagination` (B6)
11. `test(frontend): cover new DataTable shape, role filter combine, leak spy` (B7)
12. `docs: record /admin/users DataTable migration learnings` (C4)

## Progress log

(updated after each task by orchestrator)

- [x] A1 — schema change (added `lastActive: Time` to `extend type User`, added `roleId: ID` to admin `users(...)` query)
- [x] A2 — DB migration (timestamp `20260506120000`, both up.sql and down.sql, with `idx_users_last_active` index)
- [x] A3 — domain + repo + roleId join (added LastActive to gormUser, ListPage WHERE EXISTS join with user_roles, new TouchLastActive method on Repository interface)
- [x] A4 — usecase roleId thread (forwarded as last arg to ListPage; nil/empty treated identically and not parsed as UUID)
- [x] A5 — resolver wiring + gqlgen regen (model.User.LastActive auto-mapped from domain; helpers.go toUserModel updated; resolver call passes roleID)
- [x] A6 — middleware last_active write (fire-and-forget goroutine with 5s background timeout; PII-safe log via LogWarn with user_id only; AuthMiddleware constructor takes userRepo)
- [x] A7 — backend tests (4 arity fixes + 9 new tests across usecase/repo/middleware; `go test ./... -race -count=1` passes all 17 packages; `error_chain.root.stack` assertion type was `.([]string)` not `.([]any)` because chanLogHandler skips JSON round-trip)
- [x] B0 — vendor tanstack-table + shadcn primitives (added `@tanstack/react-table@^8.21.3`, `@radix-ui/react-avatar`, `react-checkbox`, `react-select`, `react-dropdown-menu`, `cmdk`; vendored 11 UI files; typecheck passes)
- [x] B1 — frontend queries.ts update (added lastActive field, $roleId variable, ADMIN_USERS_DEFAULT_VARS typed export)
- [x] B2 — frontend codegen (regenerated graphql.ts with roleId in AdminUsersQueryVariables; gitignored)
- [x] B3 — users-columns.tsx (4 columns + getInitials/formatRelativeTime helpers; non-sortable per server-side cursor ordering)
- [x] B4 — users-toolbar.tsx (search input + role single-select via Popover+Command; vendored DataTableFacetedFilter not reused due to TanStack Column coupling)
- [x] B5 — users-table.tsx (manual pagination + handlePaginationChange routes pageIndex/pageSize back to parent callbacks; React.ReactElement return type for project consistency)
- [x] B6 — AdminUsersClient.tsx rewrite (370 lines; cursor walking via Map<pageIndex, endCursor>; URL state via URLSearchParams; SSR seed in page.tsx; mechanical prop fix to existing tests; typecheck PASS)
- [x] B7 — frontend test update (10 cases in admin-users-list.test.tsx including leak spy + PII discriminator + Retry-after-error two-mock pattern; 547 tests pass; key learning: Radix `DropdownMenuItem asChild` sets `role="menuitem"`, NOT `role="link"`)
- [ ] C1 — PR review loop
- [ ] C2 — code-simplifier
- [ ] C3 — test fix loop
- [ ] C4 — docs

## Decision log (for code review iteration)

(filled by orchestrator as decisions are made)
