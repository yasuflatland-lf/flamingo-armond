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

### Widen `string` to `string | null` rather than fabricating an empty-string default

When a value has natural absence semantics — e.g. a Supabase user without an email, an unset profile bio, an optional last-viewed cardgroup — type the field as `string | null` and let consumers branch on `null`. Falling back to `""` at the layout boundary (`email: user.email ?? ""`) collapses two distinct states into one observable outcome:

```tsx
// AVOID: "" loses the distinction between "no email on this account" and "email is the empty string".
<AppShell user={{ email: user.email ?? "" }} isAdmin={isAdmin} />

// PREFER: keep the absence-bearing type all the way to the consumer.
<AppShell user={{ email: user.email }} isAdmin={isAdmin} />

interface AppShellProps {
  user: { email: string | null } | null;
  // ...
}
```

The empty-string fallback is convenient because every consumer that does `user.email.length`, `user.email.toLowerCase()`, or `<span>{user.email}</span>` "just works" — but every one of those sites silently renders an empty string for the absence case, which is rarely the intended UI. Branch explicitly: `{user.email !== null && <span>{user.email}</span>}` mirrors the runtime invariant.

**Why:** `T | null` is a single forcing function; `T = ""` is a per-consumer convention that has to be re-asserted at every read site, and any consumer that forgets is a silent bug. The same rule extends to numeric fields where `0` is a legitimate value (use `number | null`, not `number = 0`) and to dates (use `Date | null`, not the epoch). Reference: `frontend/src/components/nav/global-rail.tsx` and `frontend/src/components/nav/app-shell.tsx` (`user: { email: string | null } | null`). This rule pairs with the required-vs-optional rule above: prefer `email: string | null` (required, nullable) over `email?: string` (optional, narrower-than-it-looks).

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

### Adjacent rule: raw vs encoded twin fields on the same variant need JSDoc on both

A discriminated-union variant that intentionally exposes both a pre-encoded URL **and** the raw value the URL was built from (e.g. a navigation factory variant carrying `href: string` *and* `cardgroupId: string`) presents two `string`-typed fields the type system cannot tell apart. A future consumer doing `router.push(\`/learn/${action.cardgroupId}\`)` instead of `router.push(action.href)` silently bypasses encoding — the same correctness risk that made the encoded field exist in the first place. Pair each field with JSDoc that names its contract:

```ts
| {
    kind: "card-with-group";
    /** Pre-encoded URL — already URL-safe, route via `router.push(href)` directly. */
    href: string;
    label: "Add new card";
    /**
     * Raw, unencoded cardgroup id (e.g. for display, analytics, or as a React key).
     * Do NOT interpolate into a URL without `encodeURIComponent` — `href` is the
     * correct field for navigation.
     */
    cardgroupId: string;
  }
```

The JSDoc is the only compile-time signal that the two fields have different contracts. Removing either docstring during refactoring is a load-bearing change — treat it the same as removing the "pre-sanitized" JSDoc above. Reference: `frontend/src/components/nav/fab-action.ts` (`FabAction` `card-with-group` variant). The deeper alternative (drop the raw field entirely and force consumers to either re-parse it from `href` or expose a separate decoded helper) is acceptable, but only when no current consumer has a legitimate use for the raw form (e.g. a React `key`, an analytics event payload, a screen-reader label).

## `expect.objectContaining({ message })` is not enough — add a discriminating key

`Error.prototype.message` is an own (though non-enumerable) property on every `Error` instance. Vitest's `expect.objectContaining` uses `hasOwnProperty` for key checks. Therefore, asserting `expect.objectContaining({ message: expect.any(String) })` against a `console.error` or `console.warn` second argument will pass whether the argument is the intended structured object `{ message, err }` OR a bare `Error` regression. The test is tautologically green and the regression ships silently.

The fix is to include any **own-property key that does not exist on `Error.prototype`** as a discriminating key — `err`, `entry`, `cardgroupId`, `cardId`, `userId`, etc. all qualify. The rule is "any key the structured payload carries that a bare `Error` does not", not specifically `err`.

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

// ALSO COMPLIANT — any structured-payload-only key works as the discriminator.
// Used in `learn-client.test.tsx`: the `[learn] setLastViewedCardgroup failed`
// payload carries `cardgroupId`, which `Error.prototype` does not.
expect(consoleSpy).toHaveBeenCalledWith(
  "[learn] setLastViewedCardgroup failed",
  expect.objectContaining({ cardgroupId: CG_ID }),
);
```

**Why:** structured log arguments (`{ message, err }`) are deliberately different from a raw `Error`. The assertion must verify the structure matches what was intentionally logged, not just that some string-ish property exists. The rule applies equally to `console.error` and `console.warn` — both are used for structured payloads in this codebase (e.g. `tryGetDuplicateCardInfo` emits a `console.warn` with `{ entry }` rather than a bare string).

**How to apply:** whenever a `console.error` / `console.warn` call passes a structured object as the second argument, the assertion must reference at least one key whose presence on the payload (and absence on `Error.prototype`) the test relies on. Adding `err: expect.anything()` is the most general fix; using a domain-specific key (`cardgroupId`, `entry`, etc.) is equally compliant and often clearer because it documents what the payload actually carries. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx` `handleCreate` log assertion (uses `err`), `frontend/src/lib/apollo/graphql-errors.test.ts` (uses `entry`), and `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` persist-fail assertion (uses `cardgroupId`).

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

## Radix `asChild` Slot collapses a `null` child into an empty wrapper — gate at the parent

Radix UI primitives that accept `asChild` (e.g. `<Sheet>`, `<SidebarMenuButton>`, `<TooltipTrigger>`, `<SheetClose>`) forward props to the rendered child via the Radix `Slot` component. When the child component returns `null` (e.g. a self-suppressing `<HeaderSignInLink>` that returns `null` on `/login`), `Slot` renders nothing — but the **wrapping** Radix container (`<SidebarFooter>`, the `<nav>` block, the `<SidebarMenuItem>`) is still mounted, leaving an empty rectangle in the layout with the wrapper's padding, border, and ARIA semantics intact. There is no DOM-level signal that the slot collapsed; CSS-only review misses it because the empty container is a 1-pixel-tall gap.

```tsx
// AVOID: child self-suppresses on /login, but the SidebarFooter still mounts.
{user === null && (
  <SidebarFooter>
    <SidebarMenu>
      <SidebarMenuItem>
        <SidebarMenuButton asChild tooltip="Sign in">
          <HeaderSignInLink />  {/* returns null on /login */}
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  </SidebarFooter>
)}

// PREFER: gate the wrapper at the same place the child would self-suppress.
{user === null && pathname !== "/login" && (
  <SidebarFooter>
    {/* ...same child tree... */}
  </SidebarFooter>
)}
```

**Why:** the child's self-suppression is correct for the standalone case (e.g. when `<HeaderSignInLink>` is rendered inside something that does not have its own padding), but `asChild` Slot composition does not propagate "child rendered nothing" up to the wrapper. The two layers must agree on the suppression condition, or the layer with the broader visibility wins by default. Self-suppression in the child is convenient for one-off use; gating at the parent is correct when the parent contributes its own visual chrome.

**How to apply:** any time a child of a Radix `asChild` slot returns `null` for a known input, audit every wrapper in the chain that contributes visible chrome (padding, border, `<hr>`, ARIA landmark) and gate the outermost contributor on the same condition. Pair the parent gate with a co-located test that mounts the parent on the suppressing route and asserts the wrapper itself is absent — `expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument()` is more forcing than `queryByRole("link", { name: /Sign in/i })` because the link is gone in both the right and wrong implementations. Reference: `frontend/src/components/nav/global-rail.tsx` and `frontend/src/components/nav/logo-drawer.tsx` (`pathname !== "/login"` gate around the Sign-in `SidebarFooter` / drawer `<nav>`).

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

## `router.replace` / `router.push` must propagate preserved query params explicitly

When code rewrites the URL via `router.replace` or `router.push` from inside a flow that already received user-controlled query params (e.g. `?return=`, `?next=`, `?welcome=1`), the rewrite must explicitly carry those params forward. A naive `router.replace(\`/cards/new?cardgroup=${id}\`)` from a picker handler drops every other param the user arrived with — including the `?return=` that was supposed to control post-create navigation. The user's intent is silently lost; there is no log, no redirect-to-default, no error. The next post-create step then routes the user somewhere they did not ask for.

```tsx
// AVOID: drops every other query param the user arrived with.
function handlePickerSelect(newId: string) {
  router.replace(`/cards/new?cardgroup=${encodeURIComponent(newId)}`, { scroll: false });
}

// PREFER: rebuild via URLSearchParams and re-attach already-sanitized params.
function handlePickerSelect(newId: string) {
  const params = new URLSearchParams({ cardgroup: newId });
  if (returnTo !== null) params.set("return", returnTo);  // already sanitized upstream
  router.replace(`/cards/new?${params.toString()}`, { scroll: false });
}
```

**Why:** the URL is the single source of truth for cross-component flow state in this codebase (see `docs/frontend.md` § "/cards/new cardgroup resolution"). A handler that rewrites part of the URL is implicitly responsible for preserving every other part — anything else silently breaks the contract that the URL drives the rendered tree. The fix is to use `URLSearchParams` to assemble the new query string from the **sanitized** values already in component scope (`returnTo` post-`sanitizeReturnTo`, never raw `searchParams.get(...)`); never re-read raw user-controlled strings from `searchParams` and concatenate them into the URL — that re-opens the open-redirect surface that `sanitizeReturnTo` was meant to close.

**How to apply:** any `router.replace` / `router.push` call inside a flow that has its own `?return=` / `?next=` / shared-flow-state query param must (a) build the new URL via `URLSearchParams`, not string interpolation, and (b) re-attach the param from a previously-sanitized variable, not from `searchParams.get(...)`. Pair the implementation with a regression test that mounts the component with a `return=` value and asserts both the happy path (param preserved) and the rejection path (open-redirect value not propagated). Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `handlePickerSelect`.

## Audit collapsed helpers for branches that lose all side effects

When a refactor inlines a multi-purpose helper into a single call site and drops one of the helper's responsibilities at the same time, the surviving guard can leave a branch with no observable effect at all. The original helper combined two side effects under one shared guard:

```ts
// before — helper that ran two updates under one shape-check
function reconcileQueue(data: HandleSwipeMutation | null | undefined) {
  if (data?.handleSwipe) {
    setQueue(data.handleSwipe.nextCards);
    setPerformance(data.handleSwipe.performanceMode);
  }
}
```

A "drop the performance-mode UI" refactor inlines the helper and removes `setPerformance`. The naive transcription leaves a `data?.handleSwipe`-shaped guard with only `setQueue` inside — and the *else* arm becomes a pure silent no-op:

```ts
// after — the else arm is now a pure silent no-op; no log, no rollback, no UI signal
if (result?.data?.handleSwipe) {
  setQueue(result.data.handleSwipe.nextCards);
}
```

If `handleSwipe` resolves without a payload (mutation completed, server response missing the field), the optimistic queue silently becomes the source of truth and the operator has no way to triage the gap. The fix is an explicit `else if` that distinguishes the rollback path from the missing-data path, with a `console.warn` for operator triage:

```ts
if (result?.data?.handleSwipe) {
  setQueue(result.data.handleSwipe.nextCards);
} else if (result !== null) {
  // Mutation resolved (no .catch), but the server payload is missing handleSwipe.
  // The optimistic queue is now the source of truth; surface for operator triage.
  console.warn("[LearnClient] handleSwipe resolved without data", {
    cardId: card.id,
    cardgroupId,
  });
}
```

The `result !== null` discriminator distinguishes "mutation rejected and `.catch` returned `null`" (rollback already happened) from "mutation resolved but the payload is incomplete" (the case worth warning about). Without the discriminator, the rejected path triggers the warn redundantly.

**Why:** removing one of two side effects from a shared guard converts the guard from "do thing A and thing B together" to "do thing A or do nothing", and "do nothing" is rarely what the original guard's else case meant. The original `reconcileQueue` had nothing in the else case because both updates were always-together; once they split, the else case is suddenly a real failure mode that needs a real handler.

**How to apply:** when refactoring a helper that combines multiple side effects into a single inline call site, audit the resulting guard branches for any case that now has zero observable effect. If a branch can legitimately be reached at runtime but does nothing, replace it with an explicit `console.warn` for operator triage (or a state-rollback, depending on what "no observable effect" hides). Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.tsx` (`handleSwipe` callback) — the post-refactor `else if (result !== null)` warn after `reconcileQueue` was inlined and `setPerformance` was removed.

## Module-level singleton `Map` keyed by entity id requires a non-empty-string guard at the entry point

A module-level `Map<string, T>` (e.g. a pending-timer registry keyed by entity id) gives `""` an equal claim to be a valid key as any UUID. Two callers that independently pass `""` — a "stub id" code path, a short-circuit branch, a future caller that skips id resolution — collide silently: the second call's `cancelPending("")` cancels the first's timer, the first entry's `commitDelete` is never invoked, the cache stays in the optimistically-removed state, and the server never receives the DELETE.

The fix is a synchronous throw at the top of the public entry point. This is a programming-error guard, not user-input validation — the same level of force as the constructor-panic pattern in `.claude/rules/go-library-gotchas.md` § "Constructor panics are the right tool for non-empty config requires non-nil deps". The throw's stack trace names the bad call site; a `try/catch` that swallows it is a separate review concern at the swallowing site, not this module's problem.

```ts
export function scheduleDelete(opts: ScheduleDeleteOptions): ScheduleDeleteHandle {
  if (!opts.id) {
    throw new Error("undo-delete: id must be a non-empty string");
  }
  // ...
}
```

**Why:** `Map.get("")` and `Map.get(someRealId)` are both `O(1)` lookups. There is no runtime signal when two unrelated call sites share `""` as a key — no duplicate-key warning, no assertion, no type error. The only gate available is an explicit guard at the entry point.

**How to apply:** any module that exposes a public function accepting an id that keys into a module-level `Map` must validate `!id` (or `id === ""`) at the top of that function and throw synchronously. Pair with a co-located test that asserts the throw is synchronous and the message matches verbatim. Reference: `frontend/src/lib/undo-delete.ts` `scheduleDelete`.

## Re-scheduling a module-level singleton entry: commit prior immediately; warn-only on prior failure

When the same entity id is re-scheduled on a module-level pending registry (e.g. a fast double-swipe-to-delete on the same row triggers `scheduleDelete` twice for the same card id), naively calling `cancelPending(id)` discards the prior entry without invoking its `commitDelete`. The optimistic cache removal is now permanent — but the server never received the DELETE. Equally bad: routing the prior commit's rejection to `onCommitFailed` shows a user-facing "Could not delete. Please try again." banner for an item already gone from view. The user has no actionable retry path; the banner is misleading.

The correct pattern has two parts: (1) commit the prior entry immediately — preserving the user's intent that the prior optimistic remove is permanent — and (2) route the prior commit's rejection to `console.warn` only, NOT to `onCommitFailed`:

```ts
const existing = pending.get(opts.id);
if (existing !== undefined) {
  if (existing.toastId !== undefined) toast.dismiss(existing.toastId);
  clearTimeout(existing.timerId);
  pending.delete(opts.id);
  void existing.commitDelete().catch((err) => {
    // Prior optimistic-remove is preserved (the new schedule is the authoritative
    // intent). Do NOT call existing.onCommitFailed — the item is already gone
    // from the user's view; a banner would be misleading and has no retry path.
    console.warn(
      "[undo-delete] prior pending delete commit failed on re-schedule",
      { id: opts.id, err },
    );
  });
}
```

Per § "`expect.objectContaining({ message })` is not enough — add a discriminating key": include `id` and `err` in the structured warn payload so any test matcher discriminates against a bare `Error` regression.

**Why:** the trade-off is intentional. A server-DELETE failure on the prior entry leaves the client cache and server briefly out of sync — surfaced via a developer-tools warn. The user does NOT see a retry-prompted banner because there is no valid retry path for an entity the user has already re-deleted.

**How to apply:** any module-level pending registry that accepts a re-schedule for an already-pending id must (a) dismiss the prior UI affordance (toast, banner), (b) clear the prior timer, (c) fire the prior `commitDelete` immediately, and (d) route the prior commit's rejection to `console.warn` with a structured `{ id, err }` payload — NOT to the user-facing error callback. Reference: `frontend/src/lib/undo-delete.ts` `scheduleDelete` re-schedule branch.

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

## `useSyncExternalStore` over `useState + useEffect` for browser-store subscriptions

A hook that subscribes to an external browser store (`matchMedia`, `localStorage`, `navigator.onLine`, `document.visibilityState`, `BroadcastChannel`, etc.) and surfaces its current value to React components has two common encodings: (a) `useState` seeded with a lazy initializer plus a `useEffect` that subscribes and re-`setState` on change, or (b) `useSyncExternalStore` with `subscribe` / `getSnapshot` / `getServerSnapshot` callbacks. React's docs explicitly list (b) as the right primitive for this case; the codebase enforces (b) for every browser-store hook.

```ts
// frontend/src/hooks/use-mobile.tsx
import { useSyncExternalStore } from "react";

const MOBILE_BREAKPOINT = 768;

function subscribe(callback: () => void) {
  if (typeof window === "undefined") return () => {};
  const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

function getSnapshot() {
  if (typeof window === "undefined") return false;
  return window.innerWidth < MOBILE_BREAKPOINT;
}

function getServerSnapshot() {
  return false; // SSR default
}

export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
```

**Why:** the `useState + useEffect` shape carries three latent defects that are awkward to remove without rewriting the hook: (1) on first render the hook returns the initial state, then the effect fires post-mount and re-`setState` to the real value, producing a one-frame layout flash for any consumer whose render branches on the value; (2) consumers tend to defend against the flash by widening the type to `boolean | undefined` and collapsing the unset state with `!!isMobile` at the call site, which silently erases the "not yet measured" state; (3) the lazy initializer + effect resync is the canonical "duplicate state initialization" smell — the same value is computed once at mount and once again in the effect "in case it changed", which is evidence the wrong primitive was chosen. `useSyncExternalStore` returns the snapshot synchronously on first render and re-renders only when the subscribed callback fires, eliminating all three. Pair `subscribe` and `getSnapshot` with the `typeof window === "undefined"` guard so tests using `vitest`'s default `node` environment do not crash on import — the SSR snapshot path is what they exercise.

**How to apply:** any hook that observes a browser API and surfaces a primitive value to React MUST use `useSyncExternalStore`. Do not introduce a fresh `useState + useEffect` subscription pattern. The two standalone files in this codebase that follow the rule are `frontend/src/hooks/use-mobile.tsx` (matchMedia for layout breakpoint) and `frontend/src/lib/use-reduced-motion.ts` (matchMedia for `prefers-reduced-motion`); use either as the template. The `getServerSnapshot` value is part of the contract — pick a deterministic SSR default (`false` for "matches" predicates, `null` for absent values) and document it in a one-line comment. This rule supersedes any `useMounted` / `useIsMounted` pattern that wraps `useEffect(() => setMounted(true), [])`; see § "`next/dynamic({ ssr: false })` over a `useMounted` hook" below for the matching production-component pattern.

## `next/dynamic({ ssr: false })` over a `useMounted` hook for hydration-sensitive client-only components

A component that wraps a client-only library (`@react-spring/web`, `@use-gesture/react`, libraries that touch `window` at module scope, animation libraries that paint on first frame) needs a way to skip server-side rendering without a hydration mismatch. The historic shape was `useMounted` — a `useState(false)` plus `useEffect(() => setMounted(true), [])` that gates the client-only render branch. The recommended shape is `dynamic(() => import("./animated-card").then((m) => m.AnimatedCard), { ssr: false })`: the dynamic import handles the SSR skip at the module-loading boundary, eliminating the double render and the `useMounted` state entirely.

```tsx
// frontend/src/components/learn/swipe-card.tsx
import dynamic from "next/dynamic";

// AnimatedCard ships @react-spring/web + @use-gesture/react, which both
// require client-only execution. We load it via next/dynamic (ssr: false)
// to avoid the SSR/hydration mismatch that previously needed useMounted.
//
// Trade-off: a chunk-load failure (network blip after deploy, CDN miss)
// renders the loading fallback (`null`) without surfacing an error UI.
// The component area is briefly blank and the user must navigate away to
// recover. Accepted because: (a) chunk failures are rare in production,
// (b) the surrounding flow is forgiving, (c) adding an error fallback
// complicates the success path's rendering for an edge case. Revisit if
// telemetry shows non-trivial chunk-failure rates on the affected route.
const AnimatedCard = dynamic(() => import("./animated-card").then((m) => m.AnimatedCard), {
  ssr: false,
});
```

**Why:** `useMounted` is the canonical "you might not need an effect" anti-pattern — the effect's only job is to flip a flag, the flag's only job is to gate the render branch, and the flag exists only because the component is unsafe to render on the server. `next/dynamic` solves the same problem at the module-loading boundary so the consuming component does not need a flag at all. The double render that `useMounted` produces (first pass with the static-fallback branch, second pass with the animated branch) is also a hydration-risk band-aid: if the static fallback's DOM differs in attributes from the animated component's first paint, React still warns about a mismatch on the post-mount render. `next/dynamic` skips the server render entirely, so there is no hydration to mismatch.

**Trade-off — chunk-load failure has no error UI by default.** A `dynamic` import that fails (transient network error, CDN miss after a deploy) renders the loading fallback (`null` by default, or the `loading` callback if provided) and stays there. There is no `error` boundary callback exposed by `next/dynamic`'s API. Document the trade-off in a code comment at the call site so a future contributor does not assume the absence of an error fallback was an oversight. The acceptance criteria for skipping the error fallback are: (1) chunk failures are rare in production, (2) the surrounding flow has a graceful out (the user can navigate away or reload), and (3) adding the error UI would complicate the success path. If telemetry surfaces a non-trivial chunk-failure rate on a specific route, revisit by wrapping the dynamic component in an error boundary with a route-specific fallback.

**How to apply:** any component that previously used `useMounted` (or any equivalent `useState(false) + useEffect(() => setX(true), [])` hydration-skip pattern) MUST migrate to `next/dynamic({ ssr: false })` and document the chunk-load trade-off in a code comment. Do not introduce new `useMounted` hooks. Reference: `frontend/src/components/learn/swipe-card.tsx` (`AnimatedCard` via `next/dynamic`) replaced a prior `useMounted` gate. This rule pairs with § "`useSyncExternalStore` over `useState + useEffect`" above: both delete a `useState + useEffect` initialization pattern in favour of a primitive that React or Next.js provides for the exact use case.

## Stabilize callback identity via `useRef` mirrors when the callback reads frequently-changing state

A `useCallback` whose body reads from a stateful value listed in its dep array gets a fresh identity every time that value changes. When the callback flows down to a child that subscribes to it (e.g. a global keydown listener, an IntersectionObserver, a memoized child component), the subscription is torn down and rebuilt on every state change. The fix is to mirror the state into a ref, list the ref-owning effect as the only dep on the value, and have the callback read `ref.current`:

```tsx
// frontend/src/components/learn/swipe-card-stack.tsx — keydown listener
const activeCardRef = useRef(activeCard);
useEffect(() => {
  activeCardRef.current = activeCard;
}, [activeCard]);

// triggerSwipe stays stable across activeCard changes — no dep on activeCard.
const triggerSwipe = useCallback(
  (direction: SwipeDirection) => {
    if (!activeCardRef.current) return;
    onCardSwiped(activeCardRef.current, direction);
  },
  [onCardSwiped],
);

// keydown listener subscription does not re-register on every card.
useEffect(() => {
  function onKeyDown(event: KeyboardEvent) {
    if (!activeCardRef.current) return;
    /* ... */
  }
  window.addEventListener("keydown", onKeyDown);
  return () => window.removeEventListener("keydown", onKeyDown);
}, [triggerSwipe]);
```

The same shape applies to a `useCallback` that reads from the rendered queue / cursor / search-text but should NOT be re-created when the value advances:

```tsx
// frontend/src/app/learn/[cardgroupId]/learn-client.tsx — onSwipe callback
const queueRef = useRef(queue);
useEffect(() => {
  queueRef.current = queue;
}, [queue]);

const onSwipe = useCallback(
  async (card, direction) => {
    const remaining = queueRef.current
      .filter((c) => c.id !== card.id)
      .map(withTypename);
    /* ...build optimisticResponse with `remaining`, fire mutation... */
  },
  [cardgroupId, handleSwipe], // queue removed from deps via queueRef
);
```

**Why:** `useCallback` identity is what React uses to decide whether a child needs to re-subscribe (`useEffect` dep arrays, `React.memo` shallow-equal). A callback that re-creates on every queue update propagates that churn down through every memoized consumer, defeating the memoization. The ref mirror is a one-line indirection that removes the value from the dep array without losing access to it. The `useEffect` that writes the ref is the only place the value is observed, and writes to a ref do not trigger renders or downstream re-subscriptions.

**How to apply:** any `useCallback` that (a) reads from frequently-changing state AND (b) flows to a consumer that re-subscribes on identity change should use a ref mirror. The trigger is "is the callback's identity load-bearing for a consumer's subscription?" — if the answer is yes, mirror. The IntersectionObserver case (cursor / search / hasNextPage triplet) is documented in `.claude/rules/pagination.md` § "Stabilise `requestNextPage` via the cursor / search / hasNextPage ref triplet"; the keydown and queue cases above are non-IO uses of the same shape. Test the stability with a behavioural assertion (e.g. capture the callback in a `vi.fn()` wrapper and assert `toHaveBeenCalledTimes(1)` across multiple state updates) — see `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` (`onCardSwiped` reference stability across swipes) and `frontend/src/components/learn/swipe-card-stack.test.tsx` (keydown listener stability across `activeCard` changes).

## Derive during render instead of resetting state via a `useEffect` keyed on the trigger

When a piece of derived state depends on a snapshot of an input that may drift (e.g. "the user has not edited the payload since the last validation"), the obvious shape is a `useEffect([input])` that nulls every dependent piece of state when the input changes. The cleaner shape is to capture the snapshot the input had when the derivation last ran, and compare it to the current input during render:

```tsx
// frontend/src/app/admin/dictionary/dictionary-client.tsx
const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
// validatedPayload tracks the payloadText value that was in effect when the last
// successful validate call completed. canImport checks this against the current
// payloadText to prevent importing a stale/edited payload without re-validating.
const [validatedPayload, setValidatedPayload] = useState<string | null>(null);

async function handleValidate() {
  /* ... */
  setValidationResult(result.data.validateDictionary);
  setValidatedPayload(payloadText); // record snapshot at validation time
}

// Derived during render — no resetting useEffect needed.
const canImport =
  validationResult?.valid === true &&
  validationResult.parsedWords.length > 0 &&
  !!cardgroupId &&
  validatedPayload === payloadText; // becomes false the moment the user edits
```

**Why:** the resetting-effect shape requires three things to land together — (a) the effect itself, (b) a `biome-ignore lint/correctness/useExhaustiveDependencies` comment to acknowledge that the dep array drives a side effect rather than a synchronization, and (c) operator-mental-model debt because a reader has to trace the effect to understand when each piece of state goes back to `null`. The derive-during-render shape collapses all three into a single equality check. The double-render the effect produces (render with stale state → effect fires → re-render with nulled state) is also gone — the comparison runs in the same render the input changed.

**How to apply:** when introducing derived state that should "invalidate when X changes", first ask "can I capture a snapshot of X at the moment the derivation was last valid, and compare during render?" If yes, store the snapshot in a `useState<typeof X | null>` alongside the derived value, set the snapshot in the same handler that produced the derived value, and read the snapshot during render via `snapshot === currentX`. Reach for the resetting `useEffect` only when the invalidation also has to fire a side effect that cannot be expressed as a render-time comparison (a network round-trip, a DOM measurement). Reference: `frontend/src/app/admin/dictionary/dictionary-client.tsx` (`validatedPayload` snapshot replaces the prior three-state-reset effect). This rule pairs with § "Audit collapsed helpers for branches that lose all side effects" above: both push more work into the synchronous render path and out of post-render effects.
