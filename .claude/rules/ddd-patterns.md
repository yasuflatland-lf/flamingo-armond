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
`*string`. `IsSet()` and `Value()` expose the trinary without an out-of-band flag.
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
delete, not "keep for later". Application of the existing scope-discipline rule.

[`docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md`](../../docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md)

## Further reading (on-demand)

- [Value object Parse pattern](../../docs/backend/ddd-patterns/value-object-parse-pattern.md)
- [Caller-supplied sentinels in a field-agnostic parser](../../docs/backend/ddd-patterns/caller-supplied-sentinels-in-parser.md)
- [Trinary value object for optional profile fields](../../docs/backend/ddd-patterns/trinary-value-object.md)
- [Consumer-defined interface across package boundaries](../../docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md)
- [Aggregate behaviour methods](../../docs/backend/ddd-patterns/aggregate-behaviour-method.md)
- [Zero-value docstring on string newtypes](../../docs/backend/ddd-patterns/zero-value-docstring-on-string-newtypes.md)
- [Exported bound constants prevent message drift](../../docs/backend/ddd-patterns/exported-bound-constants-prevent-message-drift.md)
- [Helpers introduced but not wired must be deleted](../../docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md)
