# Frontend TypeScript conventions

> Applies to: `frontend/src/**/*.{ts,tsx}`. Cross-cutting type-design rules that affect correctness, security, or testability and are non-obvious from the TypeScript docs alone.

## Required `string | null` over optional `?: string | null` for security-relevant or caller-deliberate props

`prop?: string | null` and `prop: string | null` are not interchangeable. The optional form (`?`) collapses three distinct caller states into two observable outcomes — callers may omit the prop entirely, which is indistinguishable at runtime from an explicit `null` and means the type system does not force the caller to acknowledge the prop's existence. When a prop has security implications (e.g. a redirect destination, a sanitized user-supplied value) or when the calling component must make an explicit choice (pass a value or acknowledge absence), use the required form:

```ts
// AVOID: callers can omit entirely; the prop's existence is unacknowledged.
interface Props { returnTo?: string | null; }

// PREFER: callers must pass something, even if it is null.
interface Props { returnTo: string | null; }
```

The required form surfaces callers that forgot to wire the prop (compile error: "returnTo is missing") rather than silently defaulting to `undefined`. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` (`returnTo: string | null`) after a review finding that the optional form allowed callers to skip the prop and lose the sanitized redirect value without any error.

## JSDoc as the enforcer of "pre-sanitized" invariants when branded types are not used

When a function or component accepts a value that must have crossed a security boundary before being passed (e.g. "this path has been validated as an internal path"), and the project style does not use branded/nominal types, a load-bearing JSDoc comment is the only compile-time signal available. The comment must:

1. State what the caller is responsible for (e.g. "must be a pre-sanitized internal path").
2. State what the receiving side does defensively (e.g. "the receiving page also calls `sanitizeReturnTo`").
3. State what callers must NOT pass (e.g. "do not pass arbitrary user input here").

```ts
/**
 * Path to return to after creating a new cardgroup. Must be a **pre-sanitized
 * internal path** (e.g. `/cards/new`). The receiving page applies
 * `sanitizeReturnTo` defensively, but callers are responsible for not passing
 * arbitrary user input here.
 */
createReturnTo: string;
```

The JSDoc documents a two-layer defence: the caller sanitizes before passing, the receiver sanitizes again on arrival. Both layers are intentional — the "defensive" layer in the receiver is the last-resort guard against a future caller that skips pre-sanitization. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.tsx` (`createReturnTo` prop).

If the project style evolves to allow branded types, replace the JSDoc with a nominal type (e.g. `type InternalPath = string & { readonly __brand: "InternalPath" }`) and a constructor function that calls `sanitizeReturnTo`. Until then, treat the JSDoc as load-bearing — do not remove it during refactoring without adding the branded type.

## `expect.objectContaining({ message })` is not enough — add a discriminating key

`Error.prototype.message` is an own (though non-enumerable) property on every `Error` instance. Vitest's `expect.objectContaining` uses `hasOwnProperty` for key checks. Therefore, asserting `expect.objectContaining({ message: expect.any(String) })` against a `console.error` or `console.warn` second argument will pass whether the argument is the intended structured object `{ message, err }` OR a bare `Error` regression. The test is tautologically green and the regression ships silently.

The fix is to include a discriminating own-property key that exists in the structured object but NOT on `Error` instances — e.g. `err: expect.anything()`.

```ts
// AVOID — passes for both `{ message: "...", err }` AND a bare Error regression.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({ message: expect.any(String) }),
);

// PREFER — the `err` key is absent on a bare Error instance, so a regression fails.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({
    message: expect.any(String),
    err: expect.anything(),
  }),
);
```

**Why:** structured log arguments (`{ message, err }`) are deliberately different from a raw `Error`. The assertion must verify the structure matches what was intentionally logged, not just that some string-ish property exists. The rule applies equally to `console.error` and `console.warn` — both are used for structured payloads in this codebase (e.g. `tryGetDuplicateCardInfo` emits a `console.warn` with `{ entry }` rather than a bare string).

**How to apply:** whenever a `console.error` / `console.warn` call passes a structured object as the second argument, pair the `message` matcher with at least one additional key that distinguishes the object from a bare `Error`. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx` `handleCreate` log assertion and `frontend/src/lib/apollo/graphql-errors.test.ts`.

## TanStack Form `_handleSubmit` re-throws — chain `.catch()` on `form.handleSubmit()`

`@tanstack/form-core` v1.x's `_handleSubmit` wraps the user-supplied `onSubmit` in a `try { ... } catch (err) { ...done(); throw err; }` block. The re-throw is necessary for `formState.isSubmitSuccessful` to remain `false` when the submission fails. The typical JSX call shape `void form.handleSubmit()` discards the resulting rejected promise and produces a browser "Uncaught (in promise)" warning.

Chain a no-op `.catch()` at the JSX call site with an explanatory comment:

```tsx
<form onSubmit={(e) => {
  e.preventDefault();
  e.stopPropagation();
  form.handleSubmit().catch(() => {
    // The inner submit handler's .catch already logged; swallow here so the
    // re-thrown rejection (which keeps formState.isSubmitSuccessful=false correct)
    // does not surface as an unhandled browser promise rejection.
  });
}}>
```

**Why:** only the user's `onSubmit` callback's rejection propagates through `_handleSubmit` — validation failures hit `return`, not `throw`. The inner `.catch` in `onSubmit` already logs the failure, so the outer `.catch` is a deliberate, documented swallow.

**How to apply:** replace every `void form.handleSubmit()` call site with this pattern. The comment is load-bearing documentation — do not omit it. This rule pairs with the re-throw rule below; they must land together. Reference: `frontend/src/components/cardgroups/card-form.tsx`.

## Re-throw inside TanStack Form `useForm.onSubmit` to keep `isSubmitSuccessful` correct

When a parent passes a `submit` callback to a form component, swallowing a rejection inside `useForm.onSubmit` without re-throwing collapses two error states: TanStack Form sees the `onSubmit` as resolved-success and updates `isSubmitSuccessful = true`, even though the underlying mutation failed. Re-throw after logging:

```ts
onSubmit: async ({ value }) => {
  await submit(value).catch((err) => {
    console.error("[scope] submit rejected", err);
    throw err; // keep formState.isSubmitSuccessful correct
  });
},
```

**Why:** `_handleSubmit` gates `formState.isSubmitSuccessful` on whether `onSubmit` resolves or rejects. A swallowed rejection makes the form believe the submission succeeded, which can unblock navigation, clear state, or show a success banner while the mutation actually failed.

**How to apply:** every `useForm.onSubmit` that calls an external `submit` prop must re-throw after the catch/log. The re-throw is forward-safe even when the current caller wraps `submit` in its own `try/catch` — the rejection does not bubble past that boundary today. This rule pairs with the `.catch()` rule above; they must land together. Reference: `frontend/src/components/cardgroups/card-form.tsx`.

## Radix `AlertDialogAction` closes the dialog synchronously — call `e.preventDefault()` to keep it open on failure

Radix UI's `AlertDialogAction` calls `onOpenChange(false)` synchronously as soon as its `onClick` handler resolves, regardless of whether the action succeeded or failed. For a confirm action that may fail and must keep the dialog mounted (e.g. an overwrite mutation that returns a typed `BAD_USER_INPUT`), this means a failed action still closes the dialog and the user loses any inline error message.

The fix: call `e.preventDefault()` at the start of the `onClick` handler and let the consuming component decide when to close the dialog based on the outcome of the operation.

```tsx
<AlertDialogAction
  onClick={async (e) => {
    e.preventDefault(); // prevent Radix from closing on click — we close on success only
    try {
      await onConfirm();
      // caller closes the dialog (e.g. setDuplicate(null)) on success
    } catch {
      // dialog stays open; surface error inline
    }
  }}
>
  Confirm
</AlertDialogAction>
```

The cancel path uses the standard `AlertDialogCancel` component, which closes correctly without `preventDefault`. The cancel path is the catch-all close path for any non-action close (backdrop click, Escape key, explicit cancel).

**How to apply:** any `AlertDialogAction` whose `onClick` fires an async operation that can fail and must keep the dialog mounted MUST call `e.preventDefault()` at the top of the handler. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `DuplicateOverwriteDialog`.

## Discriminated union over flat DTO when consumers must branch on the variant

A factory whose output has semantically distinct shapes — e.g. "navigate to a cardgroup form", "navigate to a card form with a pre-selected cardgroup id", "navigate to a generic card form" — has two encodings available: (a) a flat DTO with a string `href` plus runtime introspection (`href.startsWith("/cards/new")`), or (b) a discriminated union with a `kind` tag. The flat DTO erases an invariant the factory already knows; the union preserves it.

```ts
// frontend/src/components/nav/fab-action.ts
export type FabAction =
  | { kind: "cardgroup"; href: "/cardgroups/new"; label: "Add new cardgroup" }
  | { kind: "card-with-group"; href: string; label: "Add new card"; cardgroupId: string }
  | { kind: "card"; href: "/cards/new"; label: "Add new card" };
```

**Why:** runtime-introspection (`startsWith`, `includes`, regex on the `href`) rots silently as new variants are added. A future `/cards/new/bulk` action would be silently picked up by `href.startsWith("/cards/new")` and routed through the wrong branch with no compile-time warning. The discriminant check forces every branching consumer to acknowledge the new variant during code review (see also "Positive allowlist over negative exclusion" below).

**How to apply:** when a factory returns one of N semantically distinct shapes AND any consumer needs to branch on which shape was returned, model the output as `{ kind: "..." } & ...` and let consumers narrow on `kind`. Use literal-type fields (`href: "/cardgroups/new"`) where the value is an invariant of the variant — the type system will reject any factory branch that produces a different string. Reference: `frontend/src/components/nav/fab-action.ts` (`FabAction`) consumed by `frontend/src/components/nav/global-fab.tsx` and `frontend/src/components/nav/header-add-card-link.tsx`. This rule generalises the route-handler-specific § "Discriminated-union response shape" in `docs/frontend.md`.

**Inverse case — do NOT use a discriminated union when only one "open" variant exists.** When a piece of state has exactly two shapes — "present with data" and "absent" — `T | null` is the right model. The `null` IS the second variant and TypeScript narrows it for free. A discriminated union shape `{ kind: "open"; data: T } | null` adds no type-safety value when no consumer ever branches on `kind` itself; it just adds boilerplate. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` (`DuplicateState = { existingCardId, existingBack, ... } | null`). Reach for the union only when variants are semantically distinct and consumers branch on which one is active.

## Positive allowlist over negative exclusion in discriminated-union narrowing

Given a discriminated union with `kind` tags, two narrowing styles are syntactically valid but semantically opposite:

```tsx
// AVOID: open-ended — every future variant silently passes through.
const href = action !== null && action.kind !== "cardgroup" ? action.href : "/cards/new";

// PREFER: closed — every future variant must be explicitly added or falls to the default.
// frontend/src/components/nav/header-add-card-link.tsx
const href =
  action !== null && (action.kind === "card-with-group" || action.kind === "card")
    ? action.href
    : "/cards/new";
```

**Why:** extension-by-default is rarely what consumer code intends. A new variant added to the union (e.g. a future `kind: "bulk-card"`) silently slips through the negative-exclusion form because `action.kind !== "cardgroup"` is true for the new variant too. The positive-allowlist form forces the addition to surface as a compile decision: either the new variant belongs in this consumer's allow list (add it) or it does not (the default branch handles it). This is the consumer-side mirror of the type-design analyzer's exhaustiveness pattern.

**How to apply:** any consumer that branches on a discriminated union's `kind` should enumerate the variants it actually wants — never the variants it does not want. The single exception is when the consumer is the type-system-enforced exhaustive switch (e.g. a `never`-defaulted `switch (action.kind)`); there, every variant is named and the compiler enforces totality. Reference: `frontend/src/components/nav/header-add-card-link.tsx` after a review finding that `action.kind !== "cardgroup"` allowed any future variant to be silently treated as a card-form destination.

## `as string` cast on regex captures under `noUncheckedIndexedAccess`

The frontend tsconfig enables `noUncheckedIndexedAccess`, which widens `RegExpExecArray[number]` to `string | undefined`. For a capture group the regex makes mandatory (i.e. the regex cannot match without producing that capture), the soundest pattern is `const id = match[1] as string;` paired with a comment naming the invariant the cast relies on:

```ts
// frontend/src/components/nav/fab-action.ts
const cardsMatch = CARDGROUP_CARDS_RE.exec(pathname);
if (cardsMatch) {
  // cardsMatch[1] is always defined when the regex matched (capture group 1 is required)
  const id = cardsMatch[1] as string;
  return { kind: "card-with-group", href: `/cards/new?cardgroup=${id}`, /* ... */ };
}
```

Avoid `String(match[1] ?? "")` — that turns `undefined` into the literal string `"undefined"`, which silently corrupts downstream URLs. Avoid the bare non-null assertion `match[1]!` because it offers no docstring anchor for the invariant: a future contributor reading `match[1]!` cannot tell whether the assertion is sound or a leftover from a refactor.

**Why:** `noUncheckedIndexedAccess` is a project-wide flag that future contributors may not be aware of. Without the comment, the `as string` cast looks superfluous and is a candidate for "cleanup" by anyone reading the code in isolation. The comment names the invariant (capture group N is required by this regex) so the cast survives review.

**How to apply:** for every regex-capture access where the capture is required by the regex, use `as string` with a one-line comment naming the required capture group. The comment is load-bearing — do not delete it during refactoring. The same pattern applies to other `noUncheckedIndexedAccess`-affected accesses (e.g. `Object.keys(o)[0]`); the rule is "explain the invariant, not just satisfy the compiler." Reference: `frontend/src/components/nav/fab-action.ts` (`cardsMatch[1] as string`, `detailMatch[1] as string`).

## Cross-module constant references in test descriptions are silent-rot coupling

A test description that names a sibling module's constant by its identifier — e.g. `"shadowed by HIDDEN_PATH_RE in global-fab.tsx"` — couples the test to that constant's exact name. A rename of the constant (or its replacement by a different mechanism, e.g. a `Set` lookup or a different regex name) leaves the test description misleading with no compile-time signal. Prefer module-relative wording that names the responsibility, not the symbol:

```ts
// AVOID: rots the moment the constant is renamed.
describe("is shadowed externally by HIDDEN_PATH_RE in global-fab.tsx", () => { ... });

// PREFER: frontend/src/components/nav/fab-action.test.ts
describe("is shadowed externally by GlobalFAB's hidden-path guard for /cardgroups/new", () => { ... });
```

**Why:** linters do not check English prose. A `grep` for the renamed constant will not find the stale test description; reviewers checking the test diff against the production diff will not flag a description that still reads naturally. The misalignment is invisible until a future reader is confused enough to investigate.

**How to apply:** when a test description must reference a sibling module's behaviour, name the **module's responsibility** (e.g. "GlobalFAB's hidden-path guard") rather than the **constant's identifier** (e.g. `HIDDEN_PATH_RE`). The same rule extends to source comments that justify a piece of code by referencing a sibling module. Reference: `frontend/src/components/nav/fab-action.test.ts` and `frontend/src/components/nav/header-add-card-link.test.tsx` after a review finding that referring to `HIDDEN_PATH_RE` by name in test prose would rot the moment the constant was replaced.
