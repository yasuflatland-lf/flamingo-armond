# Type placement in component trees: co-locate at the deepest common ancestor directory

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A shared UI type (`SwipeDirection`, `CardKind`, `DialogVariant`, …) consumed by several files in the same feature layer has two natural homes: (a) inside the highest-level route or client component that originally introduced it, or (b) a `types.ts` colocated at the **deepest directory that is an ancestor of every consumer**. The first form looks correct when only the route client and one child consume it; it rots the moment a low-level UI primitive grows a second consumer or moves into its own file.

```ts
// Anti-pattern: SwipeDirection defined inside the route-level client component.
// frontend/src/app/learn/[cardgroupId]/learn-client.tsx
export type SwipeDirection = "left" | "right" | "down";

// frontend/src/components/learn/swipe-card.tsx (low-level UI primitive)
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";
// ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
// Dependency direction is inverted: a generic, presentational component now
// depends on a specific route's client component. Any sibling feature that
// wants to reuse SwipeCard inherits the dependency on /app/learn/...

// frontend/src/components/learn/animated-card.tsx
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";

// frontend/src/components/learn/swipe-progress-overlay.tsx
import type { SwipeDirection } from "@/app/learn/[cardgroupId]/learn-client";
```

```ts
// Correct: the type lives at the deepest common ancestor directory of every
// consumer (components/learn/, since the route client also lives below the
// app/ tree and the component primitives below components/learn/). The route
// client imports the type the same way the primitives do — no special status
// for the original introducer.
//
// frontend/src/components/learn/types.ts
/**
 * Direction the learner rated a card with, derived from a horizontal or
 * downward swipe gesture (or the equivalent action-bar button / arrow key):
 * - "left"  -> Again
 * - "down"  -> Hard
 * - "right" -> Easy
 *
 * Owned by the learn component layer so low-level UI primitives
 * (SwipeCardStack, AnimatedCard, SwipeProgressOverlay, LearnActionBar)
 * do not depend on the higher-level route client.
 */
export type SwipeDirection = "left" | "right" | "down";

// frontend/src/components/learn/swipe-card.tsx
import type { SwipeDirection } from "./types";

// frontend/src/components/learn/animated-card.tsx
import type { SwipeDirection } from "./types";

// frontend/src/app/learn/[cardgroupId]/learn-client.tsx — imports the same module
import type { SwipeDirection } from "@/components/learn/types";
```

**Why:** placing a shared type in a route-level client component inverts the dependency graph. Low-level presentational primitives end up depending on the specific page that first used them, so any attempt to reuse those primitives — from a different route, a Storybook story, a test fixture, an MDX preview — drags in the entire route-client's transitive imports (the Apollo mutation hook, the toast provider context, anything that route happens to use). The collateral imports rarely break the consumer outright; they bloat the bundle for the consuming feature, slow down the dev-server compilation pass, and confuse any future contributor reading the import graph. Each consumer file now answers "why does a generic UI primitive know about the learn route?" with "it imports a type from there" — and that question never has a satisfying answer, because there is no semantic reason for the dependency to exist.

The `types.ts` colocation also gives the type a single canonical comment block — JSDoc explaining the encoded mapping (e.g. `"left" -> Again`), the source of the values (gesture / button / keyboard), and any cross-feature invariants. Comments inlined at the original-introducer site are easy to miss when a new consumer is added, and reproducing them is busywork.

**How to apply:** when a TypeScript `type` or `interface` is imported by **two or more files in different directories**, audit whether the declaration site is the deepest directory that is an ancestor of every consumer. The audit is mechanical:

```bash
grep -rln "from \"@/app/learn/\[cardgroupId\]/learn-client\"" frontend/src/components/
# If any low-level component directory shows up, the declaration is mislocated.
```

If the grep returns anything other than expected feature siblings, move the type to a colocated `types.ts` (or `<feature>-types.ts` if the directory already has a `types.ts` for an unrelated concept) and update every consumer's import path. The route-level client component becomes one consumer among equals; its history as the original introducer carries no architectural weight.

**Edge case — type used by exactly one route-level component and one direct child:** the colocated-`types.ts` form still applies, but the cost-benefit thins out. A two-consumer type that lives inside the parent is acceptable if the parent and child are likely to evolve together. The trigger for the move is the **third** consumer, or any consumer that lives in a sibling directory rather than a descendant one — anything that pushes the implicit graph out of "parent + child" territory.

**Edge case — single-export `types.ts` vs barrel files:** the `types.ts` form is deliberately a single-purpose module, not a barrel. Barrels (`index.ts` files that re-export from a directory) interact badly with tree-shaking and Next's server-component / client-component boundary classification, because every consumer of any re-exported symbol pulls in the type checker's view of every other symbol from the same file. A `types.ts` that only exports types has no runtime presence to tree-shake, so the trade-off is neutral; do not let it grow runtime values without revisiting whether the colocation is still right.

**Adjacent rule — discriminated unions over flat DTOs.** When the type being colocated is a discriminated union, the [§ "Discriminated union over flat DTO when consumers must branch on the variant"](./discriminated-union-over-flat-dto.md) rule governs the **shape** of the type, while this rule governs the **location** of its declaration. The two compose: a `HeaderCreateAction` union declared at `frontend/src/components/nav/header-create-action.ts` follows both — the shape is a discriminated union and the file is colocated at the deepest common ancestor of `logo-drawer.tsx` and `header-create-action.test.ts`. Reach for both rules when introducing a new feature-scoped type.

### Sibling subtrees with no shared route ancestor

When the same type is consumed by sibling feature directories with no common route ancestor below `frontend/src/`
(e.g. `app/cardgroups/[id]/cards/` and `app/admin/users/`), the deepest common ancestor would be `frontend/src/app/`
itself — too high. In that case, the type belongs in `frontend/src/lib/<domain>/types.ts`. This is the `lib`-level
analogue of the per-feature `types.ts` and follows the same single-purpose rule: pure type exports, no runtime values.

Reference: `frontend/src/lib/pagination/types.ts` exports `FetchNextPageInput`, consumed by both the cards
connection hook (`app/cardgroups/[id]/cards/use-cards-connection.ts`) and the admin users client
(`app/admin/users/admin-users-client.tsx`). Before extraction, the interface was declared identically in
three call sites — three places to maintain, three places for the shape to silently drift.

Reference: `frontend/src/components/learn/types.ts` (`SwipeDirection`), imported by `swipe-card.tsx`, `animated-card.tsx`, `swipe-progress-overlay.tsx`, `learn-action-bar.tsx`, `swipe-card-stack.tsx`, and the route-level `learn-client.tsx`.
