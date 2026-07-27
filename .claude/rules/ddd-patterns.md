---
paths:
  - "backend/**"
---

# DDD patterns

> Applies to: `backend/internal/domain/` and adjacent usecase + service layers.
> Learnings captured from issue #182 (Tier 3 — promote anemic-domain behaviour).

This file collects the domain-driven design patterns introduced when promoting
logic from anemic usecases into domain aggregates and value objects. For error
wrapping conventions see [`.claude/rules/error-wrapping.md`](error-wrapping.md);
for layer-graph invariants see [`.claude/rules/backend-layering.md`](backend-layering.md).

## Patterns

### Parse constructor for value objects

`Parse<Type>(s string) (<Type>, error)` is the canonical VO constructor. Trim,
validate bounds, return either the typed value or a domain sentinel. Callers at
the usecase layer translate sentinels via `translate<Type>Err` helpers.

[`docs/backend/ddd-patterns/value-object-parse-pattern.md`](../../docs/backend/ddd-patterns/value-object-parse-pattern.md)

### Caller-supplied sentinels in a field-agnostic parser

When a VO is shared by multiple fields with distinct sentinels, pass the sentinels
as parameters to keep the VO field-agnostic. The aggregate calls the parser with
the right sentinel for each field. Nil sentinels panic at the construction boundary.

[`docs/backend/ddd-patterns/caller-supplied-sentinels-in-parser.md`](../../docs/backend/ddd-patterns/caller-supplied-sentinels-in-parser.md)

### Trinary value object for optional profile fields

`Bio` encodes "no change" / "explicit clear" / "set" in a struct with a private
`*string`. `IsSet()` and `Ptr()` expose the trinary without an out-of-band flag.
`ParseBio(nil)` returns the no-change case; whitespace-only inputs collapse to
explicit-clear after trimming.

[`docs/backend/ddd-patterns/trinary-value-object.md`](../../docs/backend/ddd-patterns/trinary-value-object.md)

### Consumer-defined interface across package boundaries

`domain.FSRSScheduler` is declared inside `domain/user_card_fsrs.go` and satisfied
implicitly by `*service.FSRSScheduler`. This keeps the dependency arrow correct
(`service → domain`, not `domain → service`) while letting the aggregate call back
to the scheduler without a forbidden import. Related to the usecase → repository
variant in `docs/backend/library-gotchas/consumer-defined-narrow-repo-interface.md`.

[`docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md`](../../docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md)

### Aggregate behaviour methods (promoting anemic-domain logic)

`Role.IsSystem()`, `Rating.IsValid()`, and `UserCardFSRS.ApplyRating()` replace
usecase-side validators and inline upserts. Each method enforces its own invariant
in one place; all callers get the same check automatically.

[`docs/backend/ddd-patterns/aggregate-behaviour-method.md`](../../docs/backend/ddd-patterns/aggregate-behaviour-method.md)

### Zero-value docstring on string newtypes

`type RoleName string` allows `RoleName("")` at any caller — Go cannot seal string
newtypes. A one-line docstring ("The zero value is invalid; use `ParseRoleName` to
construct") surfaces the gap at review time and in IDE hover. Applies to
`RoleName`, `DisplayName`, and `CardText`.

[`docs/backend/ddd-patterns/zero-value-docstring-on-string-newtypes.md`](../../docs/backend/ddd-patterns/zero-value-docstring-on-string-newtypes.md)

### Exported bound constants prevent message drift

`RoleNameMax`, `DisplayNameMax`, `BioMax` are exported so usecase `translate*Err`
helpers can reference the constant instead of hardcoding the literal. Updating the
cap automatically propagates to the user-facing error message.

[`docs/backend/ddd-patterns/exported-bound-constants-prevent-message-drift.md`](../../docs/backend/ddd-patterns/exported-bound-constants-prevent-message-drift.md)

### Helpers introduced but not wired must be deleted

A speculative domain method with zero production callers at PR completion time must
be removed in the same PR. Post-flight grep confirms zero callers; a zero count means
delete, not "keep for later". Application of the existing scope-discipline rule. The
rule keys on wired-ness per symbol, not on category: a single PR introducing several
helpers of the same kind applies the grep verdict independently to each.

[`docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md`](../../docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md)

### Context-neutral docstrings on shared-context value objects

When a struct VO is consumed from both a patch-context (DTO input) and a
read-context (DB column) — e.g. `Bio` carrying `nil` / `&""` / `&"x"` in
both — the accessor docstrings must enumerate both interpretations. A
docstring that bakes in one context misleads readers arriving from the other.
Context-specific helpers (`ParseBio` for patch input, `BioFromPtr` for DB
reads) carry context-specific docstrings.

[`docs/backend/ddd-patterns/context-neutral-vo-docstrings.md`](../../docs/backend/ddd-patterns/context-neutral-vo-docstrings.md)

### Same-underlying-type pointer cast for VO bridging

When a repository row stores `*string` and the domain field is
`*<DomainStringNewtype>`, `(*domain.DisplayName)(g.DisplayName)` is a direct
pointer conversion legal under Go's same-underlying-type rule. No helper, no
nil-check needed. Does not apply to struct VOs (e.g. `Bio`), which require a
boundary helper.

[`docs/backend/ddd-patterns/same-underlying-type-pointer-cast.md`](../../docs/backend/ddd-patterns/same-underlying-type-pointer-cast.md)

### Boundary gate replaces domain re-check

A field that originates in user input and is gated at the usecase boundary
should not be re-checked in the domain aggregate. `Card.CardgroupID` is gated
by `authorizeCardgroupOrBadInput` before any `Card` is constructed; the domain
side's `Validate()` skips the presence check. The remaining `Front`/`Back`
checks survive because no boundary gate enforces them.

[`docs/backend/ddd-patterns/boundary-gate-replaces-domain-recheck.md`](../../docs/backend/ddd-patterns/boundary-gate-replaces-domain-recheck.md)

### Patch DTOs keep primitive types, not the VO

`repository.UserUpdate.Bio` stays `*string`, even though `User.Bio` is the
struct VO `Bio`. The patch contract (`nil = no change`) is about
presence/absence, which the primitive expresses literally. The usecase
translates `Bio.Ptr()` to `*string` at the DTO construction site; the
repository site stays unaware of the VO. Same shape for `CardUpdate.Front` /
`CardUpdate.Back` via `CardText.String()`.

[`docs/backend/ddd-patterns/patch-dto-primitive-not-vo.md`](../../docs/backend/ddd-patterns/patch-dto-primitive-not-vo.md)

### View-level value in the domain package

`DueCard` is a non-aggregate struct that bundles `*Card` with the viewer's
FSRS `State` and `Due` timestamp. It lives in `domain/` because
`OrderingPolicy.Apply` (a domain service) consumes it. The placement rule:
when a domain service is the consumer, keep the read-model value in `domain/`
rather than introducing a sibling `readmodel/` package whose only purpose
is to feed the service its own input.

[`docs/backend/ddd-patterns/view-level-value-in-domain-package.md`](../../docs/backend/ddd-patterns/view-level-value-in-domain-package.md)

### Caller-truncate contract for domain services

`OrderingPolicy.Apply` returns all cards it processed; each caller
truncates to its own per-session limit immediately after. The contract is
documented on the service signature; the truncate is mirrored at every call
site so a future ordering policy that emits more rows than it received cannot
exceed the caller's cap. The current caller is `LearnUsecase.NextDueCards`.

[`docs/backend/ddd-patterns/caller-truncate-contract.md`](../../docs/backend/ddd-patterns/caller-truncate-contract.md)

### Mutation response must not carry a client-managed collection

A write mutation should return only data scoped to the write (the affected
entity, an outcome enum, telemetry, validation errors). Embedding a
freshly-computed collection snapshot creates a dual-source-of-truth: the
client either overwrites its in-memory order with the server's snapshot
(causing visible reshuffles when the computation is non-deterministic) or
ignores the snapshot entirely (dead bytes). Collection refills belong in a
separate query the client triggers on its own lifecycle.

[`docs/backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md`](../../docs/backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md)

### Discovery-first due ordering (80% new / 20% prior-day review)

A default 20-card learn session is 16 uniformly-sampled never-seen cards (80%)
interleaved with 4 prior-day review slots (20%). Review slots prioritise rescue
cards whose latest rating was Again or whose stability is below
`LearnedStabilityDays`; other reviews act as filler. Rescue is day-granular up
to the exclusive JST learn-day end, while filler must be due now, and both
exclude cards swiped today via the JST start-of-day cutoff; both windows also
require a whole day to have elapsed since the card's last review, because a
sub-24h repeat can earn zero FSRS scheduling credit. SQL `random()`
decides *which* rows enter each window (selection); the injected `*rand.Rand` in
`OrderingPolicy.Apply` decides their arrangement (deterministic in tests) and
interleaves at the caller-supplied `domain.NewCardRatio` (`domain.DefaultNewCardRatio`
= 4:1 absent a stored preference). This replaces a tie-scoped
shuffle that never fired on dense real data (microsecond-precision `due` and
distinct `position` make ties structurally impossible).

[`docs/backend/ddd-patterns/discovery-first-due-ordering.md`](../../docs/backend/ddd-patterns/discovery-first-due-ordering.md)

### Append-only extension of a classification with a secondary, independently-graded source

A second source (Cambridge EVP) grades the same domain (CEFR word levels) by a
*different unit* — sense, not word — so most of its headwords are common words
that already carry an authoritative Oxford A1..C1 level. Feeding them all into
the harder-wins merge as C2 would override Oxford's correct judgments. The fix
is to make the new tier purely additive: include only keys absent from the
authoritative set, so `Oxford ∩ C2 = 0` by construction (asserted by a test) and
the harder-wins merge has no shared keys to arbitrate.

[`docs/backend/ddd-patterns/append-only-classification-extension.md`](../../docs/backend/ddd-patterns/append-only-classification-extension.md)

### Collapse "unknown" and "exists-but-hidden" into one not-found (non-disclosure gate)

When a lifecycle-gated resource (draft/published, soft-deleted, other-tenant) is read by
a caller not authorized to know it exists, return the *same* not-found for both "unknown
id" and "exists but hidden" so the endpoint cannot be used as an existence oracle. Collapse
at the lowest layer: the repository read is scoped to the visible set (`FindPublishedByID`
returns `ErrNotFound` for unknown, draft AND published-but-card-less decks alike), the
usecase maps it to a not-found data outcome, the resolver emits a state-free message.
Owner-facing reads of the same resource
may keep the distinction; erase it only across the trust boundary it protects.

[`docs/backend/ddd-patterns/notfound-collapse-non-disclosure.md`](../../docs/backend/ddd-patterns/notfound-collapse-non-disclosure.md)

### Typed bare-newtype IDs for the authorization-confusable pair

Two ownership predicates (`Cardgroup.IsOwnedBy`, `Card.BelongsToCardgroup`) share
the same `func(string) bool` shape, so a transposed id compiles and fails open at
runtime. `type CardgroupID string` / `type UserID string` (`id_types.go`) make the
two ID spaces non-swappable at compile time. They are deliberately BARE — no
`Parse` constructor — because an ID carries no domain-authored invariant: UUID
validity is guaranteed upstream by `NewID` and the DB column, and a format `Parse`
would wrongly turn a malformed id into a validation error instead of a not-found
and contradict the opaque-handle contract. Type only the authz-confusable pair and
the helper signatures with adjacent same-type params (`authorizeCardgroupOrBadInput`);
leave the shared `FindByID` interface and row structs raw `string` and cast at the
boundary (`string(id)` / `domain.UserID(user.Sub)`).

[`docs/backend/ddd-patterns/typed-id-newtype-for-authz-confusable-pair.md`](../../docs/backend/ddd-patterns/typed-id-newtype-for-authz-confusable-pair.md)

## Further reading (on-demand)

- [Value object Parse pattern](../../docs/backend/ddd-patterns/value-object-parse-pattern.md)
- [Caller-supplied sentinels in a field-agnostic parser](../../docs/backend/ddd-patterns/caller-supplied-sentinels-in-parser.md)
- [Trinary value object for optional profile fields](../../docs/backend/ddd-patterns/trinary-value-object.md)
- [Consumer-defined interface across package boundaries](../../docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md)
- [Aggregate behaviour methods](../../docs/backend/ddd-patterns/aggregate-behaviour-method.md)
- [Zero-value docstring on string newtypes](../../docs/backend/ddd-patterns/zero-value-docstring-on-string-newtypes.md)
- [Exported bound constants prevent message drift](../../docs/backend/ddd-patterns/exported-bound-constants-prevent-message-drift.md)
- [Helpers introduced but not wired must be deleted](../../docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md)
- [Context-neutral docstrings on shared-context value objects](../../docs/backend/ddd-patterns/context-neutral-vo-docstrings.md)
- [Same-underlying-type pointer cast for VO bridging](../../docs/backend/ddd-patterns/same-underlying-type-pointer-cast.md)
- [Boundary gate replaces domain re-check](../../docs/backend/ddd-patterns/boundary-gate-replaces-domain-recheck.md)
- [Patch DTOs keep primitive types, not the VO](../../docs/backend/ddd-patterns/patch-dto-primitive-not-vo.md)
- [View-level value in the domain package](../../docs/backend/ddd-patterns/view-level-value-in-domain-package.md)
- [Caller-truncate contract for domain services](../../docs/backend/ddd-patterns/caller-truncate-contract.md)
- [Mutation response must not carry a client-managed collection](../../docs/backend/ddd-patterns/mutation-response-must-not-carry-client-managed-collection.md)
- [Discovery-first due ordering (80% new / 20% prior-day review)](../../docs/backend/ddd-patterns/discovery-first-due-ordering.md)
- [Append-only extension of a classification with a secondary, independently-graded source](../../docs/backend/ddd-patterns/append-only-classification-extension.md)
- [Collapse "unknown" and "exists-but-hidden" into one not-found (non-disclosure gate)](../../docs/backend/ddd-patterns/notfound-collapse-non-disclosure.md)
- [Typed bare-newtype IDs for the authorization-confusable pair](../../docs/backend/ddd-patterns/typed-id-newtype-for-authz-confusable-pair.md)
- [Catalog/user front case asymmetry with merge reconciliation](../../docs/backend/ddd-patterns/catalog-user-deck-front-case-asymmetry.md)
