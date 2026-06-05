# Extract deterministic UX policy to a pure co-located module, out of React closures

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## Why

When a piece of UX behaviour is a deterministic data transform — a re-queue policy, an ordering rule, a dedup pass — it is **business logic, not view logic**. Defining it inside a React component (typically as the body of a `setState` updater) entangles a testable pure function with the component's render lifecycle: the only way to exercise the policy is to mount the component, drive an interaction, and read back the rendered result. The closure also invites hidden state — a `useRef`, a captured prop — to leak into what should be a self-contained `(state, args) → state` function, so the policy's correctness can no longer be reasoned about in isolation.

Extracting the transform to a pure, side-effect-free, co-located module inverts this. The module is directly unit-testable with zero React/Apollo scaffolding; the policy is reviewable in a single diff hunk independent of the component's JSX; and the component shrinks to a one-line delegation:

```tsx
setQueue((current) => advancePracticeQueue(current, card.id, outcomeFromDirection(direction)));
```

The `setState` updater carries no logic of its own — it forwards `prev` plus the event's arguments to the policy function and stores the result. There is no branch, no offset arithmetic, and no array splicing inside the component to test through the DOM.

## What

A policy module is a co-located `*.ts` file beside the component (`practice-queue.ts` next to `practice-client.tsx`) exporting:

- The **outcome/intent types** the policy operates over (`PracticeOutcome = "again" | "hard" | "easy"`).
- The **tuning constants** as named exports (`PRACTICE_REQUEUE_OFFSET = 5`), so tests import the production value rather than hardcoding a magic number — see [Import production constants in tests](import-production-constants-in-tests.md).
- A **mapping function** from the UI gesture to the domain intent (`outcomeFromDirection`: `left → again`, `down → hard`, `right → easy`).
- The **pure transition** generic over the element shape: `advancePracticeQueue<T extends { id: string }>(queue: readonly T[], cardId: string, outcome: PracticeOutcome): T[]`.

The transition's contract is the discipline that keeps it testable:

- It **never mutates the input** — `queue` is typed `readonly T[]` and every return path allocates a new array.
- It is **total** — an unknown `cardId` returns a shallow copy unchanged (defensive no-op), never throws.
- Boundary arithmetic is **self-clamping** — `again`/`hard` re-insert at `min(PRACTICE_REQUEUE_OFFSET, lengthAfterRemoval)` so a short queue stays in bounds; `easy` retires the card.

Because every input/output is a plain value, the policy carries its full test matrix as direct unit tests (18 for the practice queue) — boundary cases, the unknown-id no-op, the input-not-mutated invariant — with no rendering, no mock provider, and no fake timers.

## How to apply

When adding interaction behaviour that reduces to "given the current collection and what the user did, produce the next collection", ask whether the body of your `setState` updater is doing real work. If it branches, clamps, splices, or maps, lift it:

1. Create a co-located `*.ts` module exporting the intent type(s), tuning constant(s), the gesture→intent mapper, and the pure transition typed over `readonly T[] → T[]`.
2. Reduce the component to `setState((prev) => policyFn(prev, ...args))`. The updater is now a one-liner with nothing to test through the DOM.
3. Write the policy's test matrix as direct unit tests against the module — boundary cases, the no-op path, and a "does not mutate the input" assertion (compare against a frozen or pre-captured copy).

Reach for the in-component closure only when the transform genuinely cannot be expressed as a pure value function — when it must read live refs, issue a network call, or measure the DOM. A re-queue, an ordering pass, or a dedup is never that case.

Reference: `frontend/src/app/learn/[cardgroupId]/practice-queue.ts` (the pure module — `PracticeOutcome`, `PRACTICE_REQUEUE_OFFSET`, `outcomeFromDirection`, `advancePracticeQueue`) consumed by `frontend/src/app/learn/[cardgroupId]/practice-client.tsx`, whose `onCardSwiped` `setQueue` updater is a single delegating call. The decision to keep the policy out of the component closure was recorded in the practice-mode panel review ([#316]).

This rule is the data-transform analog of [Hook Application vs. Presentation layer separation](hook-application-vs-presentation-layer.md): that rule pushes stateful network logic into a hook and leaves render decisions in the component; this one pushes pure collection logic into a plain module and leaves the `setState` plumbing in the component. A static-source guard can lock the FSRS-safe invariant of the consuming component (no swipe mutation in practice mode) the same way as in [Static-grep regression-rule tests](static-grep-regression-guard-test.md).
