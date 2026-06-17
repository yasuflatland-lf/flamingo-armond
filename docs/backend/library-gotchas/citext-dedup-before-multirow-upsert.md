# citext unique columns: case-fold the in-memory dedup key before a multi-row ON CONFLICT upsert

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.
> Repo-side companion: [Table-parameterized bulk repository helper](table-parameterized-bulk-repo-helper.md).

When a batch importer deduplicates parsed rows in memory (a Go `map` keyed on the
conflict column) *before* handing them to a single multi-row
`INSERT ... ON CONFLICT (..., col) DO UPDATE`, the map key MUST match the DB's
uniqueness semantics for `col`. If `col` is **citext** (case-insensitive), a map
keyed on the **raw** string is wrong: `"Apple"` and `"apple"` are distinct Go map
keys but the *same* conflict key under citext.

## The failure

The shared `upsertManyTx` helper builds one multi-row statement:

```sql
INSERT INTO master_cards (..., front, ...) VALUES (...), (...)
  ON CONFLICT (master_cardgroup_id, front) DO UPDATE SET ...
```

Postgres raises **error 21000** — `ON CONFLICT DO UPDATE command cannot affect row
a second time` — when two rows in the *same* statement map to the same conflict
key. With `front` citext, two parsed rows differing only by case collapse to one
conflict key, so a raw-keyed in-memory dedup lets both through and the whole
import transaction aborts (surfaced as `INTERNAL`).

The user-card mirror (`cards.front` is plain `text`, case-sensitive) does **not**
have this problem — case-differing fronts are genuinely distinct there. The bug
is specific to the one column whose type diverges from the mirror.

## The fix: case-fold the key

```go
// backend/internal/usecase/master_card.go — ImportMasterCards dedup
lastIndex := make(map[string]int, len(words))
for i, w := range words {
    lastIndex[strings.ToLower(w.Front)] = i   // case-folded key matches citext
}
deduped := make([]textdic.ParsedWord, 0, len(words))
for i, w := range words {
    lower := strings.ToLower(w.Front)
    if lastIndex[lower] != i {
        winningBack := words[lastIndex[lower]].Back  // SAME folded key — see corollary
        // ... append a DUPLICATE diagnostic; last occurrence wins ...
        continue
    }
    deduped = append(deduped, w)
}
```

## Corollary: case-fold *every* lookup into the same map, not just the build

Once the map is keyed on `strings.ToLower(...)`, **every** read of that map must
use the same folded key. A second lookup that uses the raw key
(`lastIndex[w.Front]`) misses and returns Go's zero value `0`, silently selecting
`words[0]` instead of the intended row. This shipped briefly: the dedup *decision*
used the folded key but the `winningBack` lookup used the raw key, so the
DUPLICATE diagnostic named `words[0]`'s back text instead of the surviving row's.
The data written to the DB was correct (the `deduped` slice was built with the
folded key); only the human-facing message was wrong, which is why it slipped
past a test that asserted only the error *kind*.

## Test the diagnostic value, not just its kind

A dedup test that asserts `out.Errors[0].Kind == DUPLICATE` passes even when the
message reports the wrong winner. Pin the *content*: feed case-differing fronts
(`"Apple"`/`"apple"` with distinct backs) and assert the surviving row reached the
repo AND the diagnostic names the winning back:

```go
require.Len(t, mc.upsertCaptured, 1, "case-differing fronts must dedup to one row (citext)")
require.Equal(t, domain.CardText("second"), mc.upsertCaptured[0].Back, "last occurrence wins")
require.Contains(t, out.Errors[0].Message, "second", "diagnostic must name the winning back")
```

## Meta-lesson: concentrate tests at a mirror's divergence point

The master-card write path deliberately mirrors the user-card path
(`card.go` / `card_import.go`) almost line for line — reviewers value that
fidelity. But the two genuine divergences (`front` is citext not text; admin gate
not per-owner ownership) are exactly where mirror-by-copy is unsafe, because the
copied logic encodes the *source's* assumptions. When porting a proven path onto a
sibling aggregate, enumerate every point where the new aggregate's schema or
invariants differ from the source, and write a divergence-specific test at each —
the copy is trustworthy everywhere else, so that is where the budget belongs.
