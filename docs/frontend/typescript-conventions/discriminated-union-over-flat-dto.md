# Discriminated union over flat DTO when consumers must branch on the variant

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A factory whose output has semantically distinct shapes — e.g. "navigate to a cardgroup form", "navigate to a card form with a pre-selected cardgroup id", "open a role-creation sheet" — has two encodings available: (a) a flat DTO with a string `href` plus runtime introspection (`href.startsWith("/cards/new")`), or (b) a discriminated union with a `kind` tag. The flat DTO erases an invariant the factory already knows; the union preserves it.

```ts
// frontend/src/components/nav/header-create-action.ts
export type HeaderCreateAction =
  | { kind: "cardgroup"; label: "Add new cardgroup"; href: "/cardgroups/new" }
  | { kind: "card-with-group"; href: string; label: "Add new card"; cardgroupId: string }
  | { kind: "role"; label: "Add new role" };
```

**Why:** runtime-introspection (`startsWith`, `includes`, regex on the `href`) rots silently as new variants are added. A future `/cards/new/bulk` action would be silently picked up by `href.startsWith("/cards/new")` and routed through the wrong branch with no compile-time warning. The discriminant check forces every branching consumer to acknowledge the new variant during code review (see also "Positive allowlist over negative exclusion" below).

**How to apply:** when a factory returns one of N semantically distinct shapes AND any consumer needs to branch on which shape was returned, model the output as `{ kind: "..." } & ...` and let consumers narrow on `kind`. Use literal-type fields (`href: "/cardgroups/new"`) where the value is an invariant of the variant — the type system will reject any factory branch that produces a different string. Reference: `frontend/src/components/nav/header-create-action.ts` (`HeaderCreateAction`) consumed by `frontend/src/components/nav/logo-drawer.tsx`. This rule generalises the route-handler-specific § "Discriminated-union response shape" in [`docs/frontend/route-handler-conventions.md`](../../frontend/route-handler-conventions.md#discriminated-union-response-shape).

**Inverse case — do NOT use a discriminated union when only one "open" variant exists.** When a piece of state has exactly two shapes — "present with data" and "absent" — `T | null` is the right model. The `null` IS the second variant and TypeScript narrows it for free. A discriminated union shape `{ kind: "open"; data: T } | null` adds no type-safety value when no consumer ever branches on `kind` itself; it just adds boilerplate. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` (`DuplicateState = { existingCardId, existingBack, ... } | null`). Reach for the union only when variants are semantically distinct and consumers branch on which one is active.
