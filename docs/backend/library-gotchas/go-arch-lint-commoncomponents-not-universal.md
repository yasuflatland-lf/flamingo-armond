# go-arch-lint `commonComponents` is the only universal-import mechanism

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Only components declared under `commonComponents:` in the archfile are
implicitly importable by every other component without an explicit
`mayDependOn` entry. Every other component — including packages that feel
"cross-cutting" (`logging`, `telemetry`, `cursor`) — must be listed explicitly
in the `mayDependOn` of each component that imports it.

```yaml
# Correct: only generated DTOs in commonComponents
commonComponents:
  - graph_generated
  - graph_model

deps:
  # Each component that uses logging must say so explicitly:
  auth:               { mayDependOn: [logging, repository] }
  handler_notionsync: { mayDependOn: [logging, notion, usecase] }

  # A component that does NOT list logging cannot import it, even though
  # logging "feels" cross-cutting:
  cursor:             { anyVendorDeps: true }   # no logging here — intentional
```

## Why this matters

The failure mode is a silent dependency assumption: after adding `logging` to
one component's `mayDependOn` list and watching the lint pass, it is tempting
to assume every other component can import `logging` without an update. It
cannot — `go-arch-lint` reports a violation for the next component that tries,
even if that component's `mayDependOn` was already carefully written.

Conversely, adding a cross-cutting package to `commonComponents` to avoid the
repetition is also wrong for non-generated code: it grants every component
universal import access and removes the graph's ability to flag accidental
coupling. In the current config, `graph/generated` and `graph/model` are the
only `commonComponents` because they are gqlgen DTOs that every layer must
reference, and their content is mechanically generated with no layering
invariant attached.

## When to add to `commonComponents`

A package belongs in `commonComponents` only when:

1. It is generated output (gqlgen, goyacc, protobuf, etc.) with no layering
   constraint.
2. Every layer legitimately needs it — not "many layers need it", but
   every layer.

When in doubt, keep the package out of `commonComponents` and repeat the
`mayDependOn` entry. Repetition is cheap; a missed coupling is expensive.

## Cross-reference

See [`.claude/rules/backend-layering.md` § "Layer model"](../../../.claude/rules/backend-layering.md#layer-model)
for the full table of cross-cutting packages and why they remain outside
`commonComponents` in this repo.
