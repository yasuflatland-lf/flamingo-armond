# Extracting a sibling aggregate when a field is "about X" rather than "part of X"

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A field that started life on an aggregate often drifts in semantics until it no longer belongs there. Three structural signals indicate the field is **about** the aggregate (presentation state, per-user preference, audit trail) rather than **part of** it (identity, invariants, lifecycle-bound state):

1. The aggregate's repository file directly references **another** aggregate's table or sentinels — e.g. `repository/user.go` issues `EXISTS (SELECT 1 FROM cardgroups ...)` inside its own `UPDATE`.
2. The aggregate's package carries an import that exists **only** to classify an error against another aggregate's column — e.g. `pgconn` imported only by a `23503` classifier scoped to `users.last_viewed_cardgroup_id`.
3. A sentinel named after the other aggregate lives in the wrong namespace — e.g. `repository.ErrCardgroupNotFound` declared inside the `User` repository.

When two or more of these hold, extract the field into a sibling aggregate (own domain type, own repository, own DataLoader). The new shape:

```
domain
├── user.go               (slim)  ID / DisplayName / Bio / AvatarURL / timestamps
└── user_preference.go    (new)   UserID / LastViewedCardgroupID *string / UpdatedAt

repository
├── user.go               (slim)  no cardgroups SQL, no pgconn import
└── user_preference.go    (new)   UpsertLastViewedCardgroup, FindByUserID, FindByUserIDs,
                                  classifyUserPreferenceCardgroupFKError, ErrCardgroupNotFound
```

The GraphQL surface (`User.lastViewedCardgroup`, `setLastViewedCardgroup`) stays unchanged — the resolver picks up the new repository via DataLoader, and the frontend requires no work.

## Verify the slim boundary with grep, not by re-reading the diff

The acceptance criterion is **textual**, not behavioural. After the split, the slimmed file must contain zero references to the extracted concern:

```bash
# Both invocations must return zero matches.
grep -n "cardgroups\|pgconn" backend/internal/repository/user.go
```

Run this **on the same edit** that lands the refactor — a manual reading of `user.go` can miss a struct-tag column reference (e.g. `gorm:"column:last_viewed_cardgroup_id"`) or a one-line `toDomain` field copy that survives the larger move. The grep catches both. Once the file is clean, the grep is the load-bearing artefact that future contributors will rely on; preserve it in the PR description so reviewers can re-run it without re-deriving the predicate.

The grep is also the natural seam for a CI gate: a one-line `grep -q ... && exit 1` step in `.github/workflows/backend.yml` keeps the boundary from drifting back over future changes. Adding the gate is optional — the structural drift would be loud at review time once the file size grows — but the grep itself is non-negotiable.

## Reference

`backend/internal/repository/user.go` after the extraction contains no `cardgroups` SQL and no `pgconn` import. `backend/internal/repository/user_preference.go` carries `UpsertLastViewedCardgroup`, the `classifyUserPreferenceCardgroupFKError` helper, and the `ErrCardgroupNotFound` sentinel — all moved as a single unit. The user-preference rationale lives in [`docs/backend-graphql.md` § "`setLastViewedCardgroup` and `User.lastViewedCardgroup`"](../../backend-graphql.md#setlastviewedcardgroup-and-userlastviewedcardgroup) and [`docs/backend-db.md` § "`user_preferences` — per-user UI continuity"](../../backend-db.md#user_preferences--per-user-ui-continuity).
