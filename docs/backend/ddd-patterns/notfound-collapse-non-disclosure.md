# Collapse "unknown" and "exists-but-hidden" into one not-found (non-disclosure gate)

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

When a resource has a visibility lifecycle (draft/published, soft-deleted, other-tenant)
and the caller is **not authorized to know it exists**, the layer that answers the
request must return the *same* "not found" response for two distinct internal states:

1. the id genuinely does not exist, and
2. the id exists but the caller may not see it (it is a draft / hidden / not theirs).

Returning a *distinct* response per state — `NOT_FOUND` for unknown, `FORBIDDEN`
for draft — turns the endpoint into an **existence oracle**: a caller can enumerate
which ids map to hidden resources by diffing the two error codes. Collapsing both
into one indistinguishable not-found removes the oracle. (This is the same principle
as preferring a 404 over a 403 for resources whose very existence is privileged.)

## Mechanism — collapse at the lowest layer, carry it up unchanged

The collapse is cheapest and least bypassable when performed at the **repository**
read, not reconstructed in the usecase from a richer result:

- **Repository:** the read method is scoped to the visible set, so it cannot
  distinguish the two states for the caller. `FindPublishedByID` filters
  `WHERE id = ? AND status = 'published'` and returns `repository.ErrNotFound` for
  *both* an unknown id and a draft id (`backend/internal/repository/master_cardgroup.go`).
  A method that returned the row regardless of status and left the status check to
  the usecase would leak the row's existence to any usecase bug that forgot the check.
- **Usecase:** maps `ErrNotFound` to a not-found *data* outcome (errors-as-data),
  not an error. `ImportMaster` returns `ImportMasterOutcome{NotFound: true}, nil`
  (`backend/internal/usecase/master_catalog.go`). It never branches on draft-vs-unknown
  because the repository already erased the distinction.
- **Resolver:** returns a generic, state-free message. `ImportMasterCardgroup`
  returns `MasterNotFoundError{Message: "Master cardgroup not found"}` — no field
  reveals whether the id was unknown or a draft.

The docstrings at each layer name the invariant ("draft existence is never leaked")
so a future maintainer does not "helpfully" split the two cases back apart.

### Write paths re-read through the same gate (TOCTOU)

The gate-then-write shape has a time-of-check/time-of-use window: the caller passes
`FindPublishedByID` at the mutation boundary, then a copy/merge transaction snapshots
the deck. An unpublish landing in that window would import a now-draft deck if the
transaction re-read the master through the any-status `FindByID`. The write paths
therefore re-read through the **same** published-scoped `FindPublishedByID` inside the
transaction (`copyMasterToUserTx`, and a fetch added before `ListByMasterCardgroup` in
`MergeMasterIntoCardgroup`); the resulting `ErrNotFound` is mapped by `ImportMaster` /
`MergeMaster` to the same `NotFound` outcome as a pre-gate unknown/draft. `SeedForNewUser`
is unaffected — it already sources ids from `ListPublishedDefaultStarters`, which returns
only published decks. This closes the TOCTOU without reversing the collapse: an unpublished
master is indistinguishable from an unknown one on every path.

## The boundary — this is for unauthorized observers only

The collapse applies when the caller has no right to know the resource exists. For
an **owner-facing** read of the same resource, a distinct response is correct and
expected (the owner may legitimately learn their own draft exists, or get a specific
`FORBIDDEN` when editing someone else's published resource). Decide per audience:
erase the distinction only across the trust boundary it protects.

## Proving it

Pin the collapse where it happens, not only end-to-end:

- Repository: a test that a **draft** row returns `ErrNotFound`
  (`TestMasterCardgroupRepository_FindPublishedByID`) — this is the load-bearing
  layer; if it regresses, every consumer leaks.
- Usecase: a test that both an unknown id and a draft (both surfaced as `ErrNotFound`
  by the repo mock) yield the not-found outcome with the copy/side-effect **not run**
  (`TestImportMaster_UnknownOrDraft_ReturnsNotFoundOutcome`).
- Usecase (write-path TOCTOU): a test that a master present at the gate but unpublished
  by the time the write transaction re-reads it yields the not-found outcome with no
  cards written — proving the in-transaction re-read closes the window
  (`TestMasterDeckUsecase_MergeMasterIntoCardgroup_MasterUnpublishedMidFlight_NoImport`,
  `TestImportMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome`,
  `TestMasterCatalogUsecase_MergeMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome`).
- Resolver: a test that the not-found outcome maps to the typed error variant with a
  generic message (`TestMutationResolver_ImportMasterCardgroup_NotFound_ReturnsTypedError`).

Worked example throughout: the `importMasterCardgroup` mutation (issue #401). See also
[`result-union-errors-as-data.md`](../error-wrapping/result-union-errors-as-data.md)
for the errors-as-data outcome shape the usecase returns.
