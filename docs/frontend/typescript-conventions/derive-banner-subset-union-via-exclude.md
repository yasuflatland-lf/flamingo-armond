# Derive a banner/subset union from the outcome union with `Exclude` — don't re-declare it

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A component that renders a banner for the *failure* subset of a hook outcome needs a type covering "every outcome except success". The tempting move is to hand-write a second union; the correct move is to derive it from the outcome union with `Exclude`, so the two stay linked at the type level.

```ts
// hook: the single source of truth for the outcome shape
export type MergeFromCatalogOutcome =
  | { status: "success"; addedCount: number; updatedCount: number }
  | { status: "not_found" }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  | { status: "rejected" };

// AVOID: a parallel re-declaration with a divergent discriminant key.
type MergeBanner =
  | { type: "not_found" }                                  // `type`, not `status`
  | { type: "auth"; kind: "unauthenticated" | "forbidden" }
  | { type: "rejected" };
// Adding a new error variant to MergeFromCatalogOutcome does NOT force a matching
// MergeBanner case — the compiler flags the consuming switch but not this type, so
// the banner silently goes stale. The key rename also forces a hand mapping at every
// `setBanner({ type: ... })` call site.

// PREFER: derive the subset, keep the `status` discriminant.
type MergeBanner = Exclude<MergeFromCatalogOutcome, { status: "success" }>;
// A new error variant flows into MergeBanner automatically, and `bannerCopy(banner)`
// (which narrows on `banner.status`) fails to compile until the new case is handled.
// The non-success branch of the consumer collapses to a single `setBanner(outcome)`.
```

**Why:** a re-declared union has no type-level link to the outcome it shadows. When the outcome gains a variant, the compiler flags the consuming `switch` (because its return type is `string`, exhaustiveness is enforced there) but says nothing about the orphaned banner type — so a reader sees the banner type as "complete" when it is one variant short. A divergent discriminant key (`type` vs the codebase-wide `status`) compounds the drift: every assignment must rename fields by hand, and the inconsistency invites a wrong-key narrowing bug.

**How to apply:** when a component needs a subset of a hook/mutation outcome union (typically "all the displayable failures"), write `type X = Exclude<Outcome, { status: "success" }>` rather than re-listing the variants. Keep the discriminant key identical to the source union — every outcome union in this codebase discriminates on `status` (`ImportMasterOutcome`, `CreateCardgroupOutcome`, the `MasterMutation*Outcome` family), so a new one must too. The consumer then narrows on `status` and assigns the outcome object directly (`setBanner(outcome)`) instead of mapping field-by-field. Reference: `frontend/src/components/cardgroups/merge-from-catalog-sheet.tsx` (`MergeBanner = Exclude<MergeFromCatalogOutcome, { status: "success" }>`, consumed by `bannerCopy`). Related: [Discriminated union over flat DTO](discriminated-union-over-flat-dto.md) (why the outcome is a union at all) and [Capture `__typename` to a local before narrowing](capture-typename-before-narrowing.md).
