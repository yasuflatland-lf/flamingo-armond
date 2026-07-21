# Preview and commit must share one validator (single-source validation parity)

> Part of [`.claude/rules/go-library-gotchas.md`](../../../.claude/rules/go-library-gotchas.md). See the index for related chapters.

When a feature exposes a **preview** (dry-run / validate) path and a **commit**
(write) path over the same input, every limit the commit enforces must be
checked by a *single shared validator* that both paths call. If the two paths
grow their own copies of the limit, they drift — and a preview that reports
"✓ valid" while the commit later rejects the same payload is the worst kind of
drift, because the user trusted the preview.

## The failure mode

Card import had exactly this bug (issue #685). `validateCardImport` (preview)
parsed via `textdic` only; `importCards` (commit) parsed *and* enforced a
5,000-row cap and a per-side 500-grapheme cap (`domain.CardTextMax`). An
over-cap or over-length payload previewed as `valid: true`, then the whole
import aborted at commit time with a generic top-level `BAD_USER_INPUT` carrying
no line number. The caps lived only on the commit path; the preview never grew
them, so the two silently diverged.

## The pattern

Extract one helper that takes the parsed input and returns the violations, and
call it from **both** paths. Re-divergence becomes structurally impossible
because there is exactly one implementation.

```go
// validateImportRows is the single source of the two per-parse caps for BOTH
// validateCardImport (preview) and the commit paths (importCards,
// adminImportMasterCards). validateImportPayloadSize is its sibling for the
// third cap, on the decoded payload's byte length; keeping that check in the
// same layer is what lets one helper pair cover every cap.
func validateImportRows(words []textdic.ParsedWord) ([]validatedCard, []capViolation) { ... }
func validateImportPayloadSize(decodedBytes int) *capViolation { ... }
```

Three properties make the parity real, not just nominal:

1. **Check on the raw input, before any dedup/normalize step, so both consumers
   see the identical set.** The preview does no dedup; the commit dedups
   downstream. If the commit checked caps *after* dedup, a row that is both a
   duplicate *and* over-length would be dropped by dedup on the commit path but
   flagged on the preview path — the two would disagree on exactly the rows that
   matter. Running `validateImportRows(words)` on the raw parsed words *before*
   dedup guarantees they agree. (This is the cross-consumer corollary of the
   single-path ordering rule in
   [`classifier-check-ordering-before-pipeline-mutation.md`](../error-wrapping/classifier-check-ordering-before-pipeline-mutation.md):
   classify before you mutate the slice.)

2. **A whole-payload reject short-circuits ahead of per-row errors.** The row
   cap is a property of the whole payload, not a single line. The helper returns
   *only* the row-cap violation when the payload is over cap and never scans per
   row — the user must cut rows before any per-row error is actionable, and the
   preview and commit agree on showing the row-cap error first. Per-row length
   errors carry the row's line; the row-cap error carries `Line: 0`
   (the payload-level convention, see the `Line == 0` semantics note in
   [`docs/backend-graphql.md`](../../backend-graphql.md)).

3. **Each consumer maps the violation to its own output shape; the helper does
   not pick a wire format.** The internal `capViolation` struct carries an
   explicit `Field` ("payload" / "front" / "back") so neither caller has to
   substring-match a message — the preview maps each violation to a
   `CardImportError{Kind: CardImportErrKindHard}` and flips `Valid=false`; the
   commit maps the first violation to `ucerr.NewValidationError(v.Field, v.Message)`
   and whole-batch-aborts. Carrying the field structurally rather than parsing it
   out of the message is the same discipline as
   [`typed-classifier-over-string-prefix.md`](../error-wrapping/typed-classifier-over-string-prefix.md).

## The parity is conditioned on two textdic output properties

The "preview `valid: true` ⇒ commit succeeds" claim is **not unconditional**. A
single shared validator removes *nominal* divergence, but `validateImportRows` and
the downstream commit-path constructors are two consumers that read *different
branches* of the same `domain.ParseCardText` result — so they can still disagree
on inputs the shared helper never sees. The parity holds today only because
`textdic.Process` guarantees two properties of every `ParsedWord` it returns:

- **A1 — non-empty.** Each returned word's front and back are non-empty. The
  goyacc grammar only builds a node for the `WORD DEFINITION` production, and
  `service.go` drops any node with `n.Word == ""`, so a lone/empty side never
  reaches a caller as a word.
- **A2 — edge-trimmed.** Each returned word's front and back are already
  whitespace-trimmed (`lexWord` / `lexRun` apply `TrimRightFunc`, `skipWhiteSpace`
  drops leading whitespace), so `strings.TrimSpace(x) == x` for both sides.

Why each property is load-bearing for the parity:

- **A1 guards the preview↔commit verdict.** `validateImportRows` records *only* the
  `ErrCardFrontTooLong` / `ErrCardBackTooLong` branch of `ParseCardText` — it
  silently ignores the `ErrCardFrontRequired` / `ErrCardBackRequired` (empty)
  branch. An empty front or back therefore produces **no** cap violation, so the
  preview reports `valid: true`; but the commit builds each row via
  `domain.NewCardFromValidated`, which rejects the zero-value `CardText`, and the
  whole batch aborts with `BAD_USER_INPUT`. Preview says valid, commit rejects —
  the exact parity failure this doc exists to prevent.
- **A2 guards against a commit-only INTERNAL.** The dedup key is the **raw**
  front (`identityKey`), while the DB conflict key is the **trimmed** front
  (`uq_cards_cardgroup_front`). If a row's raw front were not trim-stable, two
  rows differing only by trailing whitespace would survive dedup as distinct raw
  keys yet collide on one trimmed conflict key inside a single `INSERT`, raising
  Postgres `21000` ("cannot affect row a second time") → an opaque `INTERNAL`.
  This is the same "key must match the storage layer's equivalence relation"
  discipline as the citext `frontMatchKey` case-fold in
  [`citext-dedup-before-multirow-upsert.md`](citext-dedup-before-multirow-upsert.md).

Both properties hold for the **only** current producer of these words
(`textdic.Process`), so neither failure is reachable today — a fact confirmed by
reading `lexer.go` / `grammar.y` and by an empirical probe over adversarial
whitespace/empty-line payloads. The point is that the guarantee is *conditional
on the parser*, not a property of the shared-validator design alone.

**Trigger for the enforcement change.** If a second input source is ever wired
into `validateImportRows` (anything other than `textdic.Process` — a direct API
payload shape, a different parser, a CSV path) that can emit an empty or
un-trimmed side, then before that source ships:

1. `validateImportRows` must record the `Required` (empty) branch as a violation
   too — not just `TooLong` — so an empty side is caught at preview time; **and**
2. the dedup key must switch from the raw front to the trimmed front (reuse the
   `CardText` VO `validateImportRows` already returns) so the dedup equivalence
   matches the DB conflict key.

Do neither preemptively: on the current `textdic`-only path both branches are
unreachable, so adding them now would be dead defensive code. This section is the
standing trigger that tells a future contributor exactly when the code defense
becomes load-bearing.

## Reuse the shared gate's parsed VOs instead of re-scanning downstream

Once the shared gate runs upstream, a later aggregate-constructor check for the
same invariant looks unreachable. Whether to keep the second scan turns on
whether an already-parsed value object is available to reuse:

- **No validated-VO constructor available → keep the re-check as defense-in-depth.**
  A constructor that only accepts raw strings re-runs the length check, but that
  scan also guards a *structural aggregate invariant* and the constructor has a
  second live failure path (e.g. `domain.NewCard` also generates an ID, whose
  `crypto/rand` failure flows through the same `if err != nil`). That is the
  "retain" verdict in
  [`defense-in-depth-classification-internal.md`](../error-wrapping/defense-in-depth-classification-internal.md).
- **A validated-VO constructor is available → reuse the VOs, do not re-scan.**
  `validateImportRows` returns the trimmed `domain.CardText` VOs it parsed for each
  row; the `Import` build loop builds the `Card` via `domain.NewCardFromValidated`
  from those VOs — one grapheme scan per row, not two. The residual
  defense-in-depth is `NewCardFromValidated`'s zero-value guard (it rejects
  `CardText("")` via the field sentinel) plus the same ID-generation failure path;
  the per-side length cap is enforced upstream by `validateImportRows`, so re-running
  `domain.ParseCardText` in the build loop would be pure redundant work. This is
  the "collapse" verdict in
  [`dead-pipeline-after-upstream-gate-collapse.md`](dead-pipeline-after-upstream-gate-collapse.md)
  applied to a *duplicated scan*: the check is not deleted, it is hoisted to its
  single source and its output reused. A source-level guard test pins the loop to
  `NewCardFromValidated` (a silent regression to `NewCard` would double-scan with
  no behavioral difference). The master-deck import mirrors this exactly through
  `domain.NewMasterCardFromValidated`, with its own source-level guard: both paths
  build their rows inside the one shared pipeline, so the VO reuse is decided
  once.

## Verification harness

The single-source property is only worth as much as a test that proves *both*
paths route through the helper. Exercise the shared validator from the preview
test (assert the preview now reports the cap as a blocking error) **and** keep
the commit path's existing reject tests green (assert the commit still aborts
with the same field/message). A preview-only test would pass even if the commit
path re-grew its own copy — pin both ends.

Worked example: `backend/internal/usecase/card_import.go` `validateImportRows`,
called by `Validate` and by the shared `runCardImport` pipeline that backs both
`Import` and `ImportMasterCards`; tests in
`backend/internal/usecase/card_import_test.go`
(`TestCardImportUsecase_ValidateDetects*` for the preview, the existing
`PayloadOverCapBadInput` / `OverLengthFrontAbortsAsValidationError` for the
commit).
