# FSRS-compatible secondary ordering: a tiebreaker between the primary key and id

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Why

A learn flow already orders due cards by an FSRS-derived key. The requirement
to surface new cards in their source document order (Notion sync) must not
disturb that primary ordering: a card that is genuinely due for review must
always precede a brand-new card, and a not-yet-due card must never jump ahead
of a due one.

The pattern that satisfies both at once is to layer the new deterministic
ordering **under** the existing primary ordering, inserting it as a tiebreaker
**between** the primary key and the final `id` tiebreaker. The due query
`findDueCardsOn` in
[`backend/internal/repository/card.go`](../../../backend/internal/repository/card.go)
emits:

```sql
ORDER BY COALESCE(ucs.due, cards.created_at) ASC, cards.position ASC, cards.id ASC
```

Because `position` sits after the due key, it only re-orders rows that are
*already tied* on `COALESCE(ucs.due, cards.created_at)`. A review card with an
earlier `ucs.due` keeps its lead over any new card regardless of `position`.
This is the whole reason the secondary ordering is FSRS-compatible: it is
strictly subordinate to the FSRS schedule. The trailing `cards.id` remains the
total-order tiebreaker so the result is deterministic even when two rows tie on
both the due key and `position`.

## What

The in-memory ordering policy mirrors the SQL tiebreaker rather than re-deriving
it. `OrderingPolicy.Apply` in
[`backend/internal/domain/service/due_card_ordering.go`](../../../backend/internal/domain/service/due_card_ordering.go)
partitions due cards into a new partition (`FSRSStateNew`) and a review
partition, then shuffles each within equal-key runs using a function **named by
the field it keys on**:

- new partition → `shuffleSamePosition` (shuffle only contiguous equal-`Position` runs)
- review partition → `shuffleSameDue` (shuffle only contiguous equal-`Due` runs)

Two functions named by their key beat one function with a `keyOn`/bool
parameter: the call site reads self-documenting and the reader never has to map
a flag value back to a field. This is the same reasoning as
[`.claude/rules/pagination.md` § "Trim direction expressed by function name, not bool flag"](../../../.claude/rules/pagination.md#trim-direction-expressed-by-function-name-not-bool-flag).

## The default-0 value is an intentional fallback

`position` is `NOT NULL DEFAULT 0`
([`20260529090000_add_position_to_cards.up.sql`](../../../backend/internal/database/migrations/20260529090000_add_position_to_cards.up.sql)).
Non-Notion cards (manual creation, seeds) all tie at `0`, so they form one
equal-`Position` run that `shuffleSamePosition` shuffles — exactly the
pre-feature "don't show the same first-N every session" behaviour. Notion sync
writes a distinct `position` per card, which makes every run size 1: no shuffle
happens and document order is reproduced deterministically. One code path
therefore serves both the ordered (Notion) and shuffled (manual) cases without
a mode flag.

## Caveat — equal-position contiguity holds only within one due-tie group

The new partition is sorted by the due key **first**, then `position`. Within a
single Notion sync every new card shares one `created_at`, so they form one
due-tie group; their positions are contiguous and the order is exact document
order. Across due-tie groups — e.g. a later re-sync whose new cards restart
their positions at 0 — `shuffleSamePosition` stays deterministic per run but
imposes no global position order across the groups. The feature accepts this
mixed-source case as out of scope to refine: the contract is per-sync document
order, not a global ordering over all unreviewed cards.

## Testing technique — prove the tiebreaker drives SELECTION, not just in-set order

A tiebreaker that only re-orders an already-selected page is weaker than one
that decides *which* rows are selected. To prove the stronger property, query
with a `limit` **smaller** than the row count, insert rows in scrambled order,
and collapse the primary key so the due key is identical for every fixture
(give them the same `created_at` and no `user_card_fsrs` row). Under those
conditions a wrong order would select different rows, so the assertion catches
selection drift, not merely display drift. The canonical example is
`TestCardRepository_FindDueCards_OrdersByPositionUnderLimit` in
[`backend/internal/repository/card_test.go`](../../../backend/internal/repository/card_test.go).

Pin the "primary key still wins" invariant directly too:
`TestCardRepository_FindDueCards_PositionDoesNotOverrideDue` gives an earlier-due
review card a high `position` and a later-due new card a low `position`, then
asserts the review card sorts ahead — proving `position` cannot override the
FSRS due key.

## Reference

- [`backend/internal/repository/card.go`](../../../backend/internal/repository/card.go) — `findDueCardsOn` emits the three-level `ORDER BY`.
- [`backend/internal/domain/service/due_card_ordering.go`](../../../backend/internal/domain/service/due_card_ordering.go) — `OrderingPolicy.Apply`, `shuffleSamePosition`, `shuffleSameDue`.
- [`backend/internal/repository/card_test.go`](../../../backend/internal/repository/card_test.go) — the selection-under-limit and primary-key-wins tests.
