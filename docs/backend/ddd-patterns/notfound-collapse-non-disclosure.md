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
  distinguish the hidden states from the unknown one for the caller.
  `FindPublishedByID` filters
  `WHERE id = ? AND status = 'published' AND EXISTS (SELECT 1 FROM master_cards …)`
  and returns `repository.ErrNotFound` for an unknown id, a draft id and a
  published deck holding zero cards alike
  (`backend/internal/repository/master_cardgroup.go`).
  A method that returned the row regardless of status and left the status check to
  the usecase would leak the row's existence to any usecase bug that forgot the check.
- **Usecase:** maps `ErrNotFound` to a not-found *data* outcome (errors-as-data),
  not an error. `ImportMaster` returns `ImportMasterOutcome{NotFound: true}, nil`
  (`backend/internal/usecase/master_catalog_import.go`, via the shared
  `verifyPublishedMaster` gate in `master_catalog.go`). It never branches on
  draft-vs-unknown because the repository already erased the distinction.
- **Resolver:** returns a generic, state-free message. `ImportMasterCardgroup`
  returns `MasterNotFoundError{Message: "Master cardgroup not found"}` — no field
  reveals whether the id was unknown or a draft.

The docstrings at each layer name the invariant ("draft existence is never leaked")
so a future maintainer does not "helpfully" split the two cases back apart.

### Write paths re-read through the same gate (TOCTOU)

The gate-then-write shape has a time-of-check/time-of-use window: the caller passes
`FindPublishedByID` at the mutation boundary, then a copy/merge transaction snapshots
the deck. An unpublish (or a delete of the deck's last card) landing in that window
would import an out-of-catalog deck if the
transaction re-read the master through the any-status `FindByID`. The write paths
therefore re-probe through the **same** catalog-scoped visibility filter
(`copyMasterToUserTx`, and a fetch before `ListByMasterCardgroup` in
`MergeMasterIntoCardgroup`); the resulting `ErrNotFound` is mapped by `ImportMaster` /
`MergeMaster` to the same `NotFound` outcome as a pre-gate unknown/draft/empty deck.

**A re-probe on a pooled connection narrows the window; it does not close it.** Such a
probe shares no snapshot with the writes that follow and holds no lock, so it admits a
deck emptied between the probe and the enumeration, and an unpublish committing after
it still lets a now-draft deck be snapshotted. The two halves of catalog visibility are
closed by different mechanisms:

- **Emptiness** — **derive the verdict from the read the write actually consumes**
  rather than from a separate probe: both write paths return `repository.ErrNotFound`
  when `len(cards) == 0` on the enumeration they are about to copy. That holds however
  the reads interleave with a concurrent last-card delete, and it needs no lock and no
  transaction-scoped repository method.
- **Unpublish** — the probe runs on the **transaction connection** and takes a
  **`FOR SHARE` lock on the master row**. `FindPublishedByIDTx` is the tx-scoped sibling
  of `FindPublishedByID`; both delegate to one private helper so the visibility filter
  cannot drift between them. `Unpublish` loads the same row `FOR UPDATE`, so the two
  serialise: an unpublish that committed first is seen by the probe, and one that
  arrives later waits until the import transaction ends. This is the read-side sibling
  of
  [TOCTOU authorization guard: lock the read rows with `FOR UPDATE`](../library-gotchas/toctou-authorization-guard-for-update-lock.md).

Note what does *not* work: moving the probe into a repeatable-read transaction. That
would pin the reads to the snapshot taken at transaction start, so a deck unpublished
afterwards would still read as published — consistency between the reads, but the wrong
answer for the write. The lock is what serialises the unpublish against the snapshot,
not the isolation level.

The lock is deliberately narrow — one row in `master_cardgroups`. The deck's master
cards stay unlocked, because the emptiness half above needs no lock and locking them
would serialise every catalog card edit against every import.

**The read-only preview verifies the same visibility, on the pooled read.**
`PreviewMergeMasterIntoCardgroup` mirrors the merge's published probe so a dry run and
the write it previews refuse the same decks: master cards outlive an unpublish, so
without the probe the preview would report a tally for a deck the confirm then rejects.
It takes the pooled `FindPublishedByID`, not the locking variant — a dry run opens no
transaction, so it has no write to serialise an unpublish against and must not hold a
row lock across a learner's think-time. The gap it cannot close is the one between the
preview response and the confirm: that spans two requests, which no transaction-scoped
lock reaches, and it surfaces as the same `NotFound` outcome on merge.

The same `ErrNotFound` reaches `SeedForNewUser`, which **skips** that starter and
seeds the rest rather than failing the batch — one deck leaving the catalog
mid-signup must not break a signup, and the invariant being protected is "never
seed an empty deck", not "seed every listed starter". Only the catalog-visibility
sentinel is skippable: any other error still aborts the whole seed, so an
infrastructure fault is never downgraded into a partial seed.

The collapse is preserved throughout: an unpublished or emptied
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
  (`TestMasterCardgroupRepository_FindPublishedByID`), and one that a **published
  row with zero cards** does too
  (`TestMasterCardgroupRepository_FindPublishedByID_EmptyDeckNotFound`) — this is
  the load-bearing layer; if it regresses, every consumer leaks.
- Usecase: a test that both an unknown id and a draft (both surfaced as `ErrNotFound`
  by the repo mock) yield the not-found outcome with the copy/side-effect **not run**
  (`TestImportMaster_UnknownOrDraft_ReturnsNotFoundOutcome`).
- Usecase (write-path TOCTOU): a test that a master present at the gate but unpublished
  by the time the write path re-reads it yields the not-found outcome with no
  cards written
  (`TestMasterDeckUsecase_MergeMasterIntoCardgroup_MasterUnpublishedMidFlight_NoImport`,
  `TestImportMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome`,
  `TestMasterCatalogUsecase_MergeMaster_MasterUnpublishedMidFlight_ReturnsNotFoundOutcome`,
  `TestMasterCatalogUsecase_MergeMaster_UnpublishCommitsAfterGate_NoCardsImported`).
- Usecase (probe wiring): a test that the published probe and the card upsert receive
  the *same* transaction handle, and that no pooled read happens on the write paths
  (`TestMasterDeckUsecase_MergeMasterIntoCardgroup_PublishedProbeRunsOnTxHandle`,
  `TestMasterDeckUsecase_CopyMasterToUser_PublishedProbeRunsOnTxHandle`). Without it a
  regression to the pooled probe passes every state-based assertion above and still
  reopens the window.
- Repository (the lock itself):
  `TestMasterCardgroupRepository_FindPublishedByIDTx_LocksRowForShare` asserts that a
  concurrent `FOR UPDATE NOWAIT` — the lock an unpublish needs — fails while the read
  transaction is open, and that a second `FOR SHARE` still succeeds so two concurrent
  imports do not serialise.
  `TestMasterCardgroupRepository_FindPublishedByIDTx_MatchesPooledVisibility` pins that
  the tx-scoped read collapses draft / card-less / unknown exactly like the pooled one.
- Real-DB concurrency:
  `TestMergeMaster_Integration_UnpublishCommitsBeforeTxBody_NoCardsImported` stages an
  unpublish inside an open transaction, starts the merge, and asserts it blocks rather
  than importing; committing the unpublish then yields the not-found outcome with zero
  cards in the destination. This is the test that fails outright if the probe moves back
  to a pooled connection.
- Preview/merge parity: `TestPreviewMergeMasterIntoCardgroup_MasterUnpublished_ReturnsNotFound`
  and `TestMasterCatalogUsecase_PreviewMergeMaster_UnpublishCommitsAfterGate_ReturnsNotFoundOutcome`
  pin that the dry run refuses exactly the decks the merge refuses.
- Resolver: a test that the not-found outcome maps to the typed error variant with a
  generic message (`TestMutationResolver_ImportMasterCardgroup_NotFound_ReturnsTypedError`).

Worked example throughout: the `importMasterCardgroup` mutation (issue #401). See also
[`result-union-errors-as-data.md`](../error-wrapping/result-union-errors-as-data.md)
for the errors-as-data outcome shape the usecase returns.
