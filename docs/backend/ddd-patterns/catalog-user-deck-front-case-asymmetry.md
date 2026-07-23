# Catalog and user deck front case asymmetry

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

The official catalog treats `"Apple"` and `"apple"` as the same headword; a learner's
own deck can hold them as two different cards. The storage asymmetry remains deliberate,
but catalog merge is a reconciliation boundary: it folds one matching user card onto the
catalog's exact casing so learner progress follows the catalog headword. User imports and
ordinary card writes remain case-sensitive.

## The asymmetry

| | Column type | Unique index | Comparison |
|---|---|---|---|
| `public.master_cards.front` (catalog) | `citext` | `uq_master_cards_cg_front (master_cardgroup_id, front)` | case-**insensitive** |
| `public.cards.front` (user deck) | `text` | `uq_cards_cardgroup_front (cardgroup_id, front)` | case-**sensitive** |

`cards.front` has been plain `text` since `20260430080000_initial_schema.up.sql`.
`master_cards.front` was migrated to `citext` later by
`20260616120000_master_cards_front_citext.up.sql`, whose comment states the intent:
changing the column type rebuilds the dependent unique index with citext's
case-insensitive operator class, "so the `ON CONFLICT (master_cardgroup_id, front)`
upsert becomes case-insensitive". citext compares case-insensitively while preserving
the stored case for display.

### Why the catalog side is case-insensitive: Notion sync reconciliation

The catalog is populated by the Notion dictionary sync, whose source is hand-edited prose.
The same headword reappears across syncs with drifting capitalisation, and every drift
must reconcile onto the *same* master row rather than accumulating near-duplicates.
Three collaborating pieces implement that, all keyed on the citext semantics:

- `frontMatchKey` (`backend/internal/usecase/notion_sync.go`) is the case-folded key
  that "mirrors the citext semantics of the `master_cards.front` column". Fronts are
  ASCII by construction (the `textdic` lexer restricts the front token to ASCII letters),
  so `strings.ToLower` agrees with Postgres `lower()` with no locale ambiguity.
- the sync plan's `dedupeByKey` step, keyed by `notionRowKey`, collapses case-variant rows
  *within one payload* before the upsert.
  This is mandatory, not cosmetic — see [hazard 2](#hazard-2--the-multi-row-upsert-needs-a-case-folded-dedup-key) below.
- `frontsToDelete` builds the prune keep-set through the same key, so a stored row whose
  case differs from the current Notion line is not mistaken for stale and deleted.

## Merge reconciliation — one case variant follows the catalog casing

**Reproducible example.**

1. A learner's deck already holds `"apple"`, with scheduling and swipe history keyed by
   that card's id.
2. A published catalog deck holds the same headword as `"Apple"`.
3. The learner runs `mergeMasterCardgroup`.

Inside the merge transaction, `CardRepository.FoldFrontCaseToTx` runs immediately before
`UpsertManyTx`. When no exact `"Apple"` row exists, it renames one case-insensitive match
to `"Apple"`: the oldest `created_at` wins, with the smallest id as the deterministic
tie-breaker. The following exact-front upsert therefore updates that row's catalog content
instead of inserting a new row. Its id does not change, so `user_card_fsrs.card_id` and
`swipe_records.card_id` continue to reference it. The database-owned `updated_at` trigger
correctly records the rename ([#1112]).

Additional case variants are deliberately left untouched. If the destination already has
both `"apple"` and an exact `"Apple"`, the exact row wins: no rename occurs, `"Apple"` is
updated, and `"apple"` remains as an independent user card. If no exact row exists but
both `"APPLE"` and `"apple"` do, only the deterministic oldest row is renamed and updated.
Merge does not attempt to reconcile the remaining variants.

`PreviewMergeMasterIntoCardgroup` uses `CardRepository.CountMatchingFrontsFold`, which
counts distinct `LOWER(front)` values against lowered catalog fronts. Multiple stored case
variants therefore predict one update, matching the one-row fold and subsequent upsert.
For an unchanged source and destination, preview `Added`/`Updated` equals the merge tally.

## Consequence 2 — a case-only admin rename never reaches learners

**Reproducible example.**

1. The catalog holds a master card stored as `"drive"`, synced from Notion.
2. An admin fixes the capitalisation in the Notion source: `drive` → `Drive`.
3. The next `POST /internal/notion-sync` run executes.

Nothing observable changes. Two independent mechanisms hold the old casing in place:

- **The upsert never rewrites `front`.** The multi-row statement built in
  `upsertManyTx` ends with
  `ON CONFLICT (<fk>, front) DO UPDATE SET back = EXCLUDED.back, updated_at = now(), position = EXCLUDED.position`.
  `front` is absent from the `DO UPDATE SET` list, so the citext conflict matches the
  stored `"drive"` row and updates only `back` / `updated_at` / `position`. The stored
  case stays `"drive"`.
- **The prune deliberately protects the old-cased row.** `frontsToDelete` compares
  through `frontMatchKey`, so the stored `"drive"` matches the incoming `"Drive"` and is
  not classified as stale. Without that case-folded comparison the row *would* be pruned
  and re-inserted with the new casing — but the sync would then destroy and recreate a
  row on every case edit, which is a worse trade than a stale display case.

Downstream, learners who import or merge that catalog deck receive `"drive"`, so a
case-only correction in the Notion source is invisible to them indefinitely.

## Migration hazards if `cards.front` were ever made `citext`

Both are machine-checked and both must be solved *before* any such migration.

### Hazard 1 — the unique-index rebuild fails on existing case-variant pairs

`ALTER COLUMN front TYPE citext` rebuilds `uq_cards_cardgroup_front` with the
case-insensitive operator class. Any user deck that already holds a case-variant pair
within one cardgroup — permitted by ordinary user writes and deliberately not fully
reconciled by [merge](#merge-reconciliation--one-case-variant-follows-the-catalog-casing)
— makes the rebuild fail. The detection query mirrors the precondition the
`master_cards` migration documents:

```sql
SELECT cardgroup_id, lower(front)
FROM public.cards
GROUP BY cardgroup_id, lower(front) HAVING count(*) > 1;
```

The `master_cards` case was tractable because the catalog is a single admin-owned dataset
whose duplicates can be reconciled at the Notion source before migrating. User decks are
not: they are per-learner data with no upstream source to fix, and each duplicate row may
carry independent `user_card_fsrs` history, so there is no mechanical answer to which row
survives and what happens to the loser's scheduling state. Any migration needs an explicit,
learner-visible reconciliation policy first.

### Hazard 2 — the multi-row upsert needs a case-folded dedup key

`upsertManyTx` emits a **single** multi-row `INSERT ... ON CONFLICT ... DO UPDATE`. Under
citext, two case-variant fronts in the same batch collapse onto one conflict target and
Postgres raises `21000`, `ON CONFLICT DO UPDATE command cannot affect row a second time`.

The catalog pipeline already carries the guard — its `dedupeByKey` step keys on
`frontMatchKey` for exactly this reason. The user-deck import pipeline has the *seam* but
not the guard: `runCardImport` (`backend/internal/usecase/card_import_pipeline.go`) routes
its dedupe through a pluggable `dedupeKey`, and the user path (`card_import.go`) supplies
`identityKey` because, as its comment states, "`cards.front` is plain text, so the conflict
key is the front verbatim". The same `identityKey` also keys `voByKey`, the map that carries each surviving
row's already-parsed value objects, so both uses must flip together. Flipping the column
type without flipping that key function turns a benign user-supplied case-variant pair
into a hard import failure.

## Verdict

**Keep the storage asymmetry, and reconcile only at catalog merge.** The catalog's exact
stored casing wins the selected row identity, while the selected card id preserves FSRS
and swipe history. An already-present exact row prevents any rename, and extra variants
remain independent.

Do not make `cards.front` case-insensitive without first addressing both migration hazards
above: a policy for every pre-existing case-variant pair and its progress rows, plus a
case-folded dedup key across every user-side `UpsertManyTx` caller. Changing only the
column type fails on existing data; changing the conflict semantics without deduplication
produces runtime `21000` failures on ordinary imports.

## Proving it

The asymmetry is pinned on both sides, so a silent flip in either direction breaks a test:

- Catalog is case-insensitive: `TestMasterCardRepository_UpsertManyTx_CaseInsensitiveFront`,
  `TestMasterCardRepository_Create_DuplicateFront_CaseInsensitive`.
- The sync's in-payload dedupe and the prune keep-set are case-insensitive:
  `TestMasterNotionSyncUsecase_CaseInsensitiveDedupe`,
  `TestMasterNotionSyncUsecase_CaseInsensitivePrune`,
  `TestMasterCard_ImportMasterCards_DeduplicatesCaseInsensitiveFront`.
- User decks retain case-sensitive uniqueness, while catalog merge folds one variant and
  keeps preview parity: `TestCardRepo_CountMatchingFrontsFold_DistinctCaseVariants`,
  `TestCardRepository_FoldFrontCaseToTx`, `TestMergeCaseFold_PreservesCardIDAndFSRS`,
  `TestMergeCaseFold_ExactVariantWins`, and `TestMergeCaseFold_OldestVariantWins`.

See also [`docs/notion-sync.md` § "Behavior"](../../notion-sync.md#behavior) for the sync's
upsert-and-prune contract that consequence 2 falls out of.
