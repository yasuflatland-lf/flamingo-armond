# PR2 — admin-edit-user optimistic concurrency (lost-update prevention)

> Tracks item 2 of issue #241. Depends on PR1 (`pr1_toctou_and_frontend_defense.md`) being merged first, since both touch `EditUser` and the `AdminEditUserOutcome` shape.

## Execution tracking

Updated: 2026-05-24 13:47:19 JST

### Task order

| # | Task | Impact | Change size | Owner cluster |
|---|---|---|---|---|
| [x] 1 | Add `users.version` migration and roundtrip coverage | High | Small | DB schema |
| [x] 2 | Thread version through domain, repository, and usecase precondition handling | High | Small | Backend core |
| [ ] 3 | Extend GraphQL schema and regenerate backend/frontend consumers | High | Small | API contracts |
| [ ] 4 | Map `ConcurrentUpdateError` through the resolver and UI save flow | High | Small | Frontend + resolver |
| [ ] 5 | Add optimistic-concurrency tests across migration, repository, usecase, and UI | High | Medium | Test coverage |

### Cluster ownership

| Cluster | Files / area | Notes |
|---|---|---|
| DB schema | `backend/internal/database/migrations/` | Owns the `users.version` column and migration roundtrip test. |
| Backend core | `backend/internal/domain/user.go`, `backend/internal/repository/user.go`, `backend/internal/usecase/admin_user.go` | Owns version propagation, compare-and-swap update, and outcome mapping. |
| API contracts | `schema/schema.graphql`, `schema/admin.graphql`, generated consumers | Owns the public token and mutation input/result contract. |
| Resolver + frontend | `backend/graph/resolver/admin.resolvers.go`, `frontend/src/app/admin/users/` | Owns the conflict response and reload/save UX. |
| Tests | backend repository/usecase tests, frontend mutation/UI tests | Owns behavior proof after implementation. |

### Parallel waves

| Wave | Depends on | Parallelizable work | Main agent role |
|---|---|---|---|
| [ ] A | PR1 merged | None | Orchestrate kickoff, assign subagents, keep scope aligned. |
| [ ] B | A | Migration + backend core + schema prep can run in parallel once the contract is fixed | Coordinate file ownership and resolve merge order. |
| [ ] C | B | Resolver and frontend updates can proceed in parallel after schema/regeneration | Verify the conflict path is wired end to end. |
| [ ] D | B, C | Backend, migration, and frontend tests can run in parallel once code lands | Collect failures, route fixes, and hand off to the commit agent. |
| [ ] E | D | Dedicated commit agent only | Do not commit from the main agent. |

### Progress log

- [x] 2026-05-24 13:47 JST - Tracking added; implementation is starting.
- [x] 2026-05-24 - Task completed: migration + database roundtrip test. Files: `backend/internal/database/migrations/20260521090000_add_version_to_users.{up,down}.sql`, `backend/internal/database/users_version_roundtrip_test.go`. Verification passed: `rtk go test ./internal/database -run TestUsersVersionDownUpRoundtrip -count=1`; full `rtk go test ./internal/database -count=1` passed.
- [x] 2026-05-24 - Task completed: GraphQL schema + backend gqlgen. Files: `schema/user.graphql`, `schema/admin.graphql`, generated backend gqlgen outputs. Verification passed: `rtk go tool gqlgen generate`, `rtk git diff --check -- schema/user.graphql schema/admin.graphql`.

## Context

`adminUserUsecase.EditUser` is a read-modify-write that writes the client's declarative input (notably the full `roleIds` set) with no version check and no row lock. Two admins editing the same user concurrently race; the second commit silently overwrites the first. The mutation is "atomic per call" but not "atomic per administrator intent".

### Why optimistic concurrency, not a row lock

`SELECT ... FOR UPDATE` only serializes the two transactions — it does NOT prevent the lost update, because `EditUser` does not recompute from a fresh read; it blind-writes the snapshot the client loaded into the edit sheet. The second admin's stale `roleIds` set still overwrites the first admin's change after the lock is released. Only an optimistic precondition (a version token the client round-trips) detects "the row changed under me" and lets the server reject the stale write.

A dedicated monotonic `version bigint` column is chosen over reusing `updated_at`:

- `updated_at` has timestamp-collision risk under high write throughput and wire-serialization fidelity issues (timezone/precision) that produce false conflicts.
- Role-only edits hit `user_roles`, which does NOT bump `users.updated_at`; covering them requires touching the users row anyway — the same work, with a less reliable token.
- A `version` integer is monotonic, clock-independent, dialect-agnostic, and lock-free (conflict detected at commit, so it scales without serialization bottlenecks regardless of table size).

The conflict surfaces as a typed `ConcurrentUpdateError` outcome variant (same "errors as data" pattern as `CannotRevokeOwnAdminRoleError`), so the client can re-fetch and prompt the admin to reload rather than silently losing data.

## Single-token design

`users.version` is bumped on EVERY `adminEditUser` write — profile-only, roles-only, or both. The edit transaction therefore ALWAYS issues an `UPDATE users SET ..., version = version + 1 WHERE id = ? AND version = ?expected`, even when no profile column changed (the `SET` carries only `version = version + 1` in that case). This makes one `users.version` token authoritative for both profile and role changes; a role-only edit by admin A bumps the version that admin B's stale `expectedVersion` is checked against.

## Changes

### 1. Migration — `backend/internal/database/migrations/`

`<timestamp>_add_version_to_users.up.sql`:

```sql
ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS version bigint NOT NULL DEFAULT 0;
```

`<timestamp>_add_version_to_users.down.sql`:

```sql
ALTER TABLE public.users DROP COLUMN IF EXISTS version;
```

- Timestamp follows the existing convention (`20260430080000_initial_schema`, `20260520090000_...`); use a value after the latest.
- Add a down/up roundtrip test per `.claude/rules/go-library-gotchas.md` → `migration-down-up-roundtrip-test.md` (prove an existing row keeps its data and gets `version = 0` after up).
- No trigger needed — the application increments `version` explicitly in the UPDATE so the precondition check and the increment are one atomic statement.

### 2. Schema — `schema/`

`schema/schema.graphql` — expose the token on `User` (the User type lives here; confirm with grep):

```graphql
type User {
  # ...existing fields...
  """Optimistic-concurrency token. Pass the value you loaded back as
  AdminEditUserInput.expectedVersion; a mismatch yields ConcurrentUpdateError."""
  version: Int!
}
```

`schema/admin.graphql`:

```graphql
"""
Returned by `adminEditUser` when the user row changed since the client loaded
it (another admin committed an edit first). The client should re-fetch the user
and retry against the fresh version rather than overwriting the newer state.
"""
type ConcurrentUpdateError implements UserError {
  message: String!
}

union AdminEditUserResult =
    AdminEditUserSuccess
  | InputValidationError
  | CannotRevokeOwnAdminRoleError
  | ConcurrentUpdateError

input AdminEditUserInput {
  displayName: String
  bio: String
  roleIds: [ID!]!
  """The version the client loaded. A mismatch at commit time yields ConcurrentUpdateError."""
  expectedVersion: Int!
}
```

Regenerate both consumers (see `schema/CLAUDE.md`):

```bash
cd backend && go tool gqlgen generate
pnpm --filter frontend codegen
```

### 3. Backend domain + repository

`backend/internal/domain/user.go` — add `Version int64` to `User`.

`backend/internal/repository/user.go`:
- `gormUser` — add `Version int64 \`gorm:"column:version"\``; map it in `userToDomain`.
- `UserUpdate` — no new field needed; the expected version is a separate parameter (it is a precondition, not a patch column).
- Replace `UpdateTx` with a version-aware form, OR add `UpdateTxVersioned(ctx, tx, id string, patch UserUpdate, expectedVersion int64) error`:

```go
// UpdateTxVersioned applies the patch and bumps version inside tx, guarded by
// the optimistic-concurrency precondition `version = expectedVersion`. ALWAYS
// issues an UPDATE (even for an empty patch) so version advances on every edit.
// Returns ErrNotFound when the id does not exist, and ErrConcurrentUpdate when
// the row exists but its version no longer matches expectedVersion.
func (r *userRepo) UpdateTxVersioned(ctx context.Context, tx *gorm.DB, id string, patch UserUpdate, expectedVersion int64) error {
	updates := userUpdates(patch)
	updates["version"] = gorm.Expr("version + 1")
	res := tx.WithContext(ctx).Model(&gormUser{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: update user tx versioned")
	}
	if res.RowsAffected == 0 {
		// Distinguish not-found from version conflict.
		var count int64
		if cerr := tx.WithContext(ctx).Table("users").Where("id = ?", id).Count(&count).Error; cerr != nil {
			return eris.Wrap(cerr, "repository: update user tx versioned: existence probe")
		}
		if count == 0 {
			return ErrNotFound
		}
		return ErrConcurrentUpdate
	}
	return nil
}
```

Add the sentinel (plain `errors.New`, so `errors.Is` works):

```go
// ErrConcurrentUpdate is returned when an optimistic-concurrency precondition
// (version match) fails: the row exists but was modified since the caller read it.
var ErrConcurrentUpdate = errors.New("repository: concurrent update")
```

### 4. Backend usecase — `admin_user.go`

- `AdminEditUserInput` — add `ExpectedVersion int64`.
- `AdminEditUserOutcome` — add `ConcurrentUpdate bool` (keep the XOR invariant: exactly one of `User` / `Validation` / `CannotRevokeOwnAdmin` / `ConcurrentUpdate`).
- In the tx callback, the profile write becomes unconditional (always bump version):

```go
if uerr := u.users.UpdateTxVersioned(ctx, tx, id, patch, input.ExpectedVersion); uerr != nil {
	if isContextDone(uerr) {
		return uerr
	}
	if errors.Is(uerr, repository.ErrConcurrentUpdate) {
		return uerr // mapped to ConcurrentUpdate outcome after the tx
	}
	return eris.Wrap(uerr, "usecase: admin user edit: update profile")
}
```

- `mapAdminEditMutationError` — add a leading case: `errors.Is(err, repository.ErrConcurrentUpdate)` → signal the `ConcurrentUpdate` outcome. Because the mapper returns `(*InputValidationInfo, error)`, either extend its signature to return the outcome, or (cleaner) detect `ErrConcurrentUpdate` in `EditUser` before calling the mapper, similar to the `guardHit` short-circuit from PR1:

```go
if err != nil {
	if isContextDone(err) {
		return AdminEditUserOutcome{}, err
	}
	if errors.Is(err, repository.ErrConcurrentUpdate) {
		return AdminEditUserOutcome{ConcurrentUpdate: true}, nil
	}
	info, perr := mapAdminEditMutationError(err)
	// ...
}
```

- The `profilePatch` bool is no longer a write-gate (we always UPDATE to bump version), but keep it to decide whether `userUpdates(patch)` carries any real column — purely informational now; the UPDATE runs regardless.

### 5. Backend resolver — `backend/graph/resolver/admin.resolvers.go`

Add the mapping case before the success case:

```go
if outcome.ConcurrentUpdate {
	return model.ConcurrentUpdateError{
		Message: "This user was changed by someone else. Reload and try again.",
	}, nil
}
```

Map `outcome.User.Version` into the returned `User` model (the resolver's `toUserModel` must carry the new field).

### 6. Frontend — `frontend/src/app/admin/users/`

- `queries.ts` — add `version` to the `User` selection used to load the sheet, and add `ConcurrentUpdateError { message }` to the `AdminEditUserMutation` result selection. Add `$expectedVersion: Int!` to the mutation and pass it in `input`.
- `admin-user-row.ts` (`AdminUserListItem`) — carry `version: number`.
- `admin-user-profile-sheet.tsx`:
  - Capture the loaded `user.version` (it is stable for the open sheet; no extra state needed beyond reading `user.version` at save time).
  - Include `expectedVersion: user.version` in the mutation `input`.
  - Success-path switch — add a `ConcurrentUpdateError` case that sets a dedicated message and triggers a re-fetch (call the parent's reload, e.g. via `onSaved` semantics or a new `onConflict` prop that re-queries and re-opens with fresh data). Minimal version: `setSaveError("This user was changed by someone else. Reload and try again.")` plus invoking the existing list/detail refetch.
- Add a constant `ERR_CONCURRENT = "This user was changed by someone else. Reload and try again."`.

### 7. Tests

- Backend usecase: a case where `UpdateTxVersioned` returns `ErrConcurrentUpdate` → outcome `{ConcurrentUpdate: true}`, asserted via `assertAdminEditUserOutcomeXOR`.
- Backend repository: integration test proving two concurrent versioned updates — first commits (version 0→1), second with `expectedVersion=0` gets `ErrConcurrentUpdate`; and a not-found case (`expectedVersion` match but id absent) returns `ErrNotFound`.
- Migration roundtrip test (item 1 above).
- Frontend: a `ConcurrentUpdateError` mutation result renders the concurrent-edit copy and triggers the reload path. A successful save still passes `expectedVersion`.

## Verification

```bash
# backend
cd backend && go tool gqlgen generate && go vet ./... && go build ./...
go test -race ./internal/usecase/... ./internal/repository/...
go tool go-arch-lint check --project-path .

# frontend
pnpm --filter frontend codegen
pnpm --filter frontend typecheck && pnpm --filter frontend lint
pnpm --filter frontend test -- admin-user-profile-sheet

# gates (repo root) — both empty
grep -rlP "[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]" --include="*.go" --include="*.tsx" --include="*.graphql" backend/internal frontend/src schema
grep -rnE "PR[0-9]+" backend/internal frontend/src schema
```

## PR sizing

Larger than PR1 but should stay under the 800-line production-diff ceiling: migration (~6 lines), schema (~15), domain/repo (~50), usecase (~25), resolver (~10), frontend (~40). Regenerated `graph/generated`, `graph/model`, `frontend/src/generated` are mechanical and excluded from the intent-review budget. If the frontend reload-on-conflict UX grows, split that into its own commit but keep it in this PR.

## Open questions to resolve at implementation time

- **`User.version` exposure scope**: exposing `version` on the shared `User` type makes it visible to all `User` consumers. Confirm no non-admin query leaks it inappropriately (it is a harmless integer, but the schema doc should state its purpose). Alternatively expose it only on the admin user query's projection if the schema allows a narrower field.
- **Reload UX on conflict**: decide whether `ConcurrentUpdateError` auto-refetches and re-stages the sheet (losing the admin's unsaved edits) or shows a banner with a manual "Reload" affordance (preserves edits until the admin chooses). Default recommendation: manual reload affordance, so the admin can copy their intended changes before discarding.
