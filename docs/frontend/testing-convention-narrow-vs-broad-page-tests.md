# Testing convention: narrow vs broad page tests

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

All tests live under `frontend/__tests__/` using Vitest + Testing Library. Two naming conventions split responsibility:

**Narrow tests** (`<feature>-<flow>.test.tsx`) isolate a single user-facing flow introduced by a feature PR. Examples: `cards-pagination.test.tsx` (pagination + fetchMore only), `cards-bulk-delete.test.tsx` (selection and delete only), `admin-users-roles.test.tsx` (assign/revoke roles only), `admin-roles-crud.test.tsx` (create/update/delete only), `admin-layout.test.tsx` (admin gate only). Each narrow test is shipped by the feature PR that introduced its flow, locking in expected behaviour.

**Broad tests** (`<page>.test.tsx`) guard the page-level composition and integration points across PRs. Examples: `cardgroups-list.test.tsx`, `cardgroups-detail.test.tsx`, `cards-list.test.tsx`, `admin-roles.test.tsx`, `admin-users-list.test.tsx`. Each broad test covers SSR auth gate, initial render, empty state, and error boundaries — without duplicating the narrow test's flow-specific assertions.

**Anti-pattern**: Do not name a flow-specific test with a page-level name. If a feature PR introduces a flow that is the only flow on its page, still name the test `<page>-<flow>.test.tsx` to reserve the `<page>.test.tsx` slot for the future broad test.

**Shared utilities** live under `frontend/__tests__/utils/` and `frontend/__tests__/fixtures/`:

- `mock-apollo-paginated.ts` — one-mock-per-fetchMore helper with inline documentation. Provides `installApolloMockLeakSpy`, which captures `console.warn` calls matching `"No more mocked responses for the query"`; calling `assertNoLeaks()` in `afterEach` throws if any were recorded, catching double-fetch regressions.
- `fixtures/users.ts` and `fixtures/cardgroups.ts` — shared test data.

**JSDoc on shared utilities is part of the contract.** Reviewers should treat shared helpers under `__tests__/utils/` as if a new contributor will copy their usage examples verbatim — runnable copy-paste-ready snippets, not approximations. Document which fields each state-mutating knob touches (e.g. a `setError` that does not clear a previously-set user) so chained calls have predictable observed behaviour.

### The narrow / broad split is a contract

Putting a flow-detail assertion in a broad-named file (e.g. a role-checkbox toggle inside `admin-users-list.test.tsx`) silently locks in implementation detail and forces the broad test to break on every refactor of the narrow flow. The narrow / broad split is not a guideline — it is a contract: broad tests assert only page-level composition (SSR auth gate, initial render, empty state, error boundaries); flow-specific assertions belong in their narrow companion file.

A page can host **both** a co-located `<page>.test.tsx` (next to the source under `src/app/...`) and a `__tests__/<page>.test.tsx` (broad scope) file. The co-located test focuses on the page's local refactor surface (e.g. stubbing the client component); the `__tests__/` file mounts the full tree end-to-end. Coverage between the two MUST be deconflicted manually — the author of any new broad test must read both before adding assertions, otherwise duplicate redirect / auth-gate cases accumulate across the two files.

### Suspense refactor: test the `Content` component directly, not the outer `Page`

When a route is refactored to use `loading.tsx` + `<Suspense>`, the outer `page.tsx` default export becomes a thin shell that returns a `<Suspense>` element — it no longer calls `gqlFetch` directly. Broad-page tests that were previously mounted via `render(await Page())` and asserted on gqlFetch-driven content will fail because the element tree now contains a suspended `<Content />` child that Vitest's renderer cannot resolve synchronously.

The correct pattern after a Suspense refactor is two test groups:

1. **Outer shell test** — calls `await Page()` and asserts that the element is a `<Suspense>` whose `fallback` is the skeleton and whose `children` is the `Content` component reference. No gqlFetch mock required.
2. **Content tests** — call `await Content()` directly (using the named export added for this purpose) and render the result. These test all data-layer branches: empty state, populated state, UNAUTHENTICATED redirect, non-auth error rethrow.

```ts
import CardgroupsPage, { CardgroupsContent } from "./page";

it("returns a <Suspense> boundary with <CardgroupsSkeleton /> as fallback", async () => {
  const jsx = await CardgroupsPage();
  expect(jsx.type).toBe(Suspense);
  expect(jsx.props.fallback.type).toBe(CardgroupsSkeleton);
  expect(jsx.props.children.type).toBe(CardgroupsContent);
});

it("passes connection edges to CardgroupsClient", async () => {
  vi.mocked(gqlFetch).mockResolvedValue(makeConnection([{ id: "cg-1", name: "Spanish Vocab" }]) as never);
  const jsx = await CardgroupsContent();
  render(jsx);
  expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
});
```

Pages that co-locate a `page.test.tsx` next to the source file (rather than under `frontend/__tests__/`) use the same split. See [`auth-outside-suspense-boundary.md`](./rsc-error-handling/auth-outside-suspense-boundary.md) for why `CardgroupsContent` is exported as a named export rather than inlined.

### RSC test rendering pattern

Tests for Next.js 15+ async server components render by `await`ing the page function and passing its element tree to `render(...)`. The page params argument is `Promise<{...}>`, not a plain object:

```ts
render(await CardgroupDetailPage({ params: Promise.resolve({ id: "cg-1" }) }));
```

Pre-15 patterns that pass `{ params: { id } }` directly will not type-check or will misbehave at runtime.

`createSupabaseServerClient` is server-only. The repo has no MSW; the `server-only` import is stubbed at the Vitest config level (`vitest.config.ts`) so any module that pulls it in transitively does not crash the test runner. Page-level tests no longer stub `@/lib/supabase/server` for the auth gate — every protected page (including `app/admin/*` and `app/login/page.tsx`) reads the middleware-forwarded `x-auth-status` header rather than calling `getUser()` at render time (see below). A direct `vi.mock("@/lib/supabase/server", ...)` factory survives only in route-handler and library tests that genuinely invoke the server client — e.g. `frontend/src/app/auth/callback/route.test.ts` and `frontend/src/lib/apollo/server.test.ts`.

Protected pages read the middleware-forwarded `x-auth-status` header via `readAuthContext(await headers())` and do not call `getUser()` at render time. Their tests mock `next/headers`:

```ts
vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));
```

Non-authenticated cases override the mock per-test:

```ts
vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));
```

Do not reach for `@/lib/supabase/server` stubs when testing a page that uses `readAuthContext`; the `getUser()` path is not exercised and the factory mock is unused overhead.

### Auth-gate migration touches both the narrow and broad test

When a page's auth gate changes (e.g. from calling `getUser()` in the RSC body to reading `readAuthContext(await headers())`), the change requires updating **two** test files, not one:

1. The co-located narrow test `frontend/src/app/<route>/page.test.tsx`.
2. The broad integration test `frontend/__tests__/<feature>.test.tsx` that instantiates the same page.

A grep scoped to `frontend/src/app/` misses the broad `frontend/__tests__/` test. Both must be updated — swapping the `@/lib/supabase/server` `getUser` mock for a `next/headers` mock — and both must pass before the change is complete. The broad test is not a duplicate: it exercises the page as a full route, not just its RSC function in isolation.

Concrete examples: `app/cardgroups/page.tsx` is covered by both `src/app/cardgroups/page.test.tsx` and `__tests__/cardgroups-list.test.tsx`; `app/cardgroups/[id]/edit/page.tsx` by both `src/app/cardgroups/[id]/edit/page.test.tsx` and `__tests__/cardgroup-edit.test.tsx`.

Failure mode if the broad test is missed: it still mocks `@/lib/supabase/server` while the page no longer calls `getUser()`, and the unmocked `headers()` call throws `` `headers` was called outside a request scope ``, failing CI with a confusing error unrelated to the auth logic under test.

### Assert queue contents via prop capture, not via rendered text

A mock component that renders only the top item of a list (e.g. `SwipeCardStack` rendering only the active card) hides everything below the surface. Asserting `screen.queryAllByText("duplicate-front")` can detect a duplicate at position 0 but not at position 3 — the duplicate is in the queue but not rendered.

The solution is to accumulate every `cards` prop snapshot passed to the mock in a module-level array, then assert on the array directly:

```ts
// Declared at module scope (outside describe), reset in beforeEach.
const capturedCardSnapshots: SwipeCardData[][] = [];

vi.mock("@/components/learn/swipe-card-stack", () => ({
  SwipeCardStack: (props: { cards: SwipeCardData[]; /* ... */ }) => {
    capturedCardSnapshots.push([...props.cards]);  // snapshot every render
    // render only the active card (top of queue)
    const activeCard = props.cards[0];
    if (!activeCard) return <div><p>Session complete</p></div>;
    return <div><p>{activeCard.front}</p></div>;
  },
}));

beforeEach(() => {
  capturedCardSnapshots.length = 0;  // reset — do not bleed across tests
});
```

The dedup test then waits for the expected queue length in the snapshot array and asserts on card ids directly, without touching the rendered DOM:

```ts
await waitFor(() => {
  const latest = capturedCardSnapshots.at(-1);
  expect(latest?.length).toBe(6);  // 5 originals + 1 prefetched unique
});

const mergedIds = capturedCardSnapshots.at(-1)!.map((c) => c.id);
// The duplicate was not added a second time.
expect(mergedIds.filter((id) => id === "q-3").length).toBe(1);
// The fresh card is present exactly once.
expect(mergedIds.filter((id) => id === "p-unique").length).toBe(1);
```

**Why not `screen.findAllByText`:** the mock renders only the top card's text, so `queryAllByText` and `findAllByText` are blind to any duplicate at position 1+. The capture array is the only way to assert on the full queue contents without swipe-exhausting the stack card-by-card. Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` — `capturedCardSnapshots` used by the prefetch dedup test.

### `vi.spyOn` requires `vi.restoreAllMocks()` in `afterEach`

`vi.clearAllMocks()` resets call history but does **not** restore original implementations. Spies installed via `vi.spyOn(console, "warn")` (or any other property spy) accumulate across tests when only `clearAllMocks` runs in `beforeEach` — the second test sees a spy left behind by the first, and the third sees both. Symptoms are confusing: a `console.warn` spy installed in test A still records calls made by test B's setup, the `expect(spy).not.toHaveBeenCalled()` assertion in test B fails for reasons the test cannot explain.

Pair them:

```ts
beforeEach(() => {
  vi.clearAllMocks();    // wipes call history of all known mocks
});
afterEach(() => {
  vi.restoreAllMocks();  // restores any spy installed via vi.spyOn back to the real impl
});
```

Reference: `frontend/src/app/layout.test.tsx` (lines 125–139). This applies to any test file that calls `vi.spyOn(...)` on a global (`console`, `Date`, `crypto`) or a module export.

### `vi.useRealTimers()` in `afterEach` is a defensive guard for fake-timer leaks

A single test that calls `vi.useFakeTimers()` and then `throw`s — or simply forgets to call `vi.useRealTimers()` at the end — leaks fake timers into every subsequent test in the same file. Symptoms: `userEvent` interactions hang because their internal `setTimeout(0)` never fires, and `MockedProvider`'s async resolution never settles because the microtask runner is paused. The whole suite then times out with no useful stack pointing at the offending test. Pin a defensive `vi.useRealTimers()` in `afterEach` in any file that touches fake timers (or *might* touch them via a refactor):

```ts
afterEach(() => {
  vi.useRealTimers();
  // ...other teardown
});
```

Two corollaries:

1. **Real timers + `waitFor` is usually the right tool for `setTimeout`-based UI** (e.g. a 2 s self-dismissing banner). Switching to fake timers `AFTER` the `setTimeout` is already pending does not migrate the existing real-timer handle into the fake queue — the timer keeps running on real time while `vi.advanceTimersByTime(...)` does nothing. Either install fake timers before the component mounts, or stay on real timers and `waitFor(..., { timeout: 3000 })`.
2. **Avoid `mockImplementation(() => {})` on outer console spies that wrap an Apollo leak spy.** The leak spy from `installApolloMockLeakSpy` is itself a `console.warn` spy with `mockImplementation`; installing a second `vi.spyOn(console, "warn")` afterwards makes the second spy *outer* (it intercepts first), and an `outer.mockImplementation(() => {})` call swallows every warning before it ever reaches the inner leak spy — the inner spy then records nothing and `assertNoLeaks()` becomes a no-op. Track non-leak warnings via `expect(consoleWarnSpy).toHaveBeenCalledWith(...)` instead, and keep the outer spy in pass-through (call-recording) mode. Restore the outer spy first in `afterEach`, then the leak spy — LIFO order matches the install order. See [`docs/pagination/capture-mockedprovider-warn-leaks.md`](../pagination/capture-mockedprovider-warn-leaks.md) for the full chain.

### Apollo Client v4 testing migration gotchas

Test code copy-pasted from v3 examples will fail typecheck against v4:

- **`addTypename` prop removed.** v4's `MockedProviderProps` no longer exposes `addTypename`; `__typename` handling is automatic. Drop the prop instead of carrying it forward as `addTypename={false}`.
- **`MockedResponse` import path is `@apollo/client/testing`**, not `@apollo/client/testing/react`. The `/testing/react` subpath does not re-export `MockedResponse`.
- **`useMutation` returns `[mutateFn, { loading, error, data, reset, ... }]`.** A stub like `[vi.fn(), { loading: false }]` works for a client that only reads `loading` but breaks invisibly when the client later reads any other field. Stubs of `useMutation` should mirror the full second-tuple shape — or use `MockedProvider` instead and skip the manual stub entirely.

### TypeScript strict array indexing in fixtures

With `noUncheckedIndexedAccess` on, `arr[i]` is typed as `T | undefined`, so `cardsFixture[0].front` does not type-check. Resolve at the access site with a non-null assertion plus a Biome-ignore comment justifying the literal-array safety (`cardsFixture[0]!.front`), or shape the fixture as a tuple via `as const` so the type system knows the length statically. The non-null-assertion route is preferred for variable-length fixtures.

### Extract pure logic out of client components for `@vitest-environment node` tests

When a client component contains a pure mapping function — e.g. a pathname → action lookup, a route → label lookup, a state → CSS-class lookup — lifting that function into a sibling module enables testing it under `// @vitest-environment node`. The node environment skips jsdom setup, React mock plumbing, and `next/navigation` mocks; the test runs as a plain function-call assertion against a string input.

```ts
// frontend/src/components/nav/header-create-action.ts — pure helper, no React, no next/navigation imports.
export function resolveHeaderCreateAction(pathname: string): HeaderCreateAction | null { /* ... */ }

// frontend/src/components/nav/header-create-action.test.ts
// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveHeaderCreateAction } from "./header-create-action";

describe("resolveHeaderCreateAction", () => {
  it("returns Add new cardgroup href for exact /cardgroups", () => {
    expect(resolveHeaderCreateAction("/cardgroups")).toEqual({ kind: "cardgroup", /* ... */ });
  });
});
```

The component then becomes a thin shell that calls the helper:

```tsx
// frontend/src/components/nav/logo-drawer.tsx
const action = resolveHeaderCreateAction(pathname);
if (action === null) return null;
```

**Why:** pure logic + jsdom is wasted overhead — every test pays for the DOM environment to assert a string-in-string-out result. Node tests are faster (no jsdom bootstrap), clearer (no `vi.mock` of `next/navigation`), and the production code gets a forcing function to keep the helper React-free. The same testability-extraction principle is documented for the backend in [`docs/backend/library-gotchas/testable-startup-helpers.md`](../backend/library-gotchas/testable-startup-helpers.md).

**How to apply:** when a client component's render function or hook callback contains branching logic that depends only on its arguments (not on React state, refs, or router objects), lift that logic into a sibling `.ts` file with no React or Next.js imports, and write its tests under `// @vitest-environment node`. The component imports the helper and calls it. Reference: `frontend/src/components/nav/header-create-action.ts` consumed by `logo-drawer.tsx`; the test file `header-create-action.test.ts` runs under the node environment while the component tests stay on jsdom.

### Co-located component-level test for prop-guard branches the integration path cannot reach

When a presentational component grows a new prop with a guard (`completedCount != null && completedCount > 0 && (...)`) plus a singular/plural ternary, the page-level integration test (e.g. `LearnClient` mounted with a `MockedProvider`) only exercises the populated branch — the parent always passes a real number, so `undefined` and `=== 0` are unreachable from above. A co-located test next to the component is the cheapest, most-targeted home for those branches: no `MockedProvider`, no Apollo wiring, no router mocks, just `render(<Component {...props} />)` and `screen.getByText(...)` for each branch:

```tsx
// frontend/src/components/learn/swipe-card-stack.test.tsx
// @vitest-environment jsdom
describe("SwipeCardStack — Session-complete count line", () => {
  it("renders no count line when completedCount is undefined", () => {
    render(<SwipeCardStack {...baseProps} />);
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });
  it("renders no count line when completedCount is 0", () => {
    render(<SwipeCardStack {...baseProps} completedCount={0} />);
    expect(screen.queryByText(/You reviewed/)).not.toBeInTheDocument();
  });
  it("renders singular when completedCount is 1", () => {
    render(<SwipeCardStack {...baseProps} completedCount={1} />);
    expect(screen.getByText("You reviewed 1 card in this batch.")).toBeInTheDocument();
  });
  it("renders plural when completedCount is 2", () => {
    render(<SwipeCardStack {...baseProps} completedCount={2} />);
    expect(screen.getByText("You reviewed 2 cards in this batch.")).toBeInTheDocument();
  });
});
```

**Why:** the page-level test is shaped by the page's own contract (real query results, real navigation), so adding a "what if this prop were 0" branch to it forces the test to invent a synthetic state the page never emits. The co-located component test owns the prop's contract directly — every branch the prop can reach is a test, every test is one render. The cost is tiny (≤30 lines per component, no Apollo overhead), and the coverage is exhaustive in a way the integration test cannot be. This is the presentational-component analogue of the helper-extraction pattern above: there, the helper is pure code; here, the prop's branch table is the "pure" surface.

**How to apply:** when a presentational component under `frontend/src/components/` adds a prop whose values branch the rendered output AND the parent at the integration site only passes a single value (or a narrow value range), add a co-located `<component>.test.tsx` next to the component covering every branch the prop can take. Use jsdom (the component renders DOM) and skip `MockedProvider` / `next/navigation` mocks unless the component actually uses them. Reference: `frontend/src/components/learn/swipe-card-stack.test.tsx` covering `completedCount` undefined / 0 / 1 / 2 branches that `LearnClient` integration tests cannot reach.

### `toHaveClass(...)` is token-based — prefer it over `className.toContain(...)`

`@testing-library/jest-dom`'s `toHaveClass("sticky")` checks whether the element's class list **contains that token**,
whereas `className.toContain("sticky")` is a raw substring match that produces false positives: `"not-sticky"` passes
a `toContain("sticky")` assertion.

Always use `toHaveClass` and its negative form `not.toHaveClass`:

```ts
expect(outer).toHaveClass("sticky");
expect(outer).not.toHaveClass("fixed");
expect(outer).not.toHaveClass("inset-x-0");
```

When narrowing from `Element | null` after a `.not.toBeNull()` guard, TypeScript still treats the value as `Element | null`.
Satisfy `toHaveClass`'s non-null requirement with an explicit guard rather than a non-null assertion:

```ts
const outer = container.firstElementChild as HTMLElement | null;
expect(outer).not.toBeNull();
if (outer === null) return;          // narrows type; toHaveClass never sees null
expect(outer).toHaveClass("sticky");
```

### Do not assert CSS layout or visual class names in jsdom tests

jsdom has no rendering engine. A class like `sticky`, `bottom-0`, `border-red-600`, or `justify-center` is never applied visually — it is only a string in the DOM. Asserting it tests nothing about user-observable behavior: the element is never sticky in a jsdom test, no color is ever shown, and no flex layout is ever computed.

Tests that assert Tailwind utility names break on design changes (palette swaps, layout renames, CSS-custom-property migrations) with no corresponding behavioral regression, creating maintenance noise without safety benefit.

**Delete** CSS layout and visual class assertions. The behavioral contracts they shadow — keyboard clickability, focus order, accessible names, callback invocation — are already covered by `userEvent` interaction tests and ARIA attribute checks.

Exception: when responsive visibility is the behavioral contract under test — one element visible only on desktop (`max-lg:hidden`) and its sibling visible only on mobile (`lg:hidden`) — the mutual exclusivity between the two is the contract, not a cosmetic detail. Assert both directions on both elements so a copy-paste error that applies the same class to both cannot slip through:

```ts
// frontend/src/app/login/page.test.tsx
const formBrandHeader = container.querySelector("[data-testid='form-brand-header']");
const brandPanel = container.querySelector("[data-testid='brand-panel']");
expect(formBrandHeader?.className).toMatch(/lg:hidden/);
expect(formBrandHeader?.className).not.toMatch(/max-lg:hidden/);
expect(brandPanel?.className).toMatch(/max-lg:hidden/);
expect(brandPanel?.className).not.toMatch(/(^|\s)lg:hidden(\s|$)/);
```

### Anchor regex patterns when asserting Tailwind class presence

When asserting that `lg:hidden` is NOT on an element, the regex `/lg:hidden/` falsely matches `max-lg:hidden` because it is a substring. Use a word-boundary anchor so the pattern matches only the standalone token:

```ts
// WRONG — matches max-lg:hidden, producing a false negative
expect(el?.className).not.toMatch(/lg:hidden/);

// CORRECT — matches only the standalone lg:hidden token
expect(el?.className).not.toMatch(/(^|\s)lg:hidden(\s|$)/);
```

Apply the anchored form on every negative `not.toMatch` for a class that could appear as a suffix or prefix of another class in the same element's class list. Positive `toMatch(/lg:hidden/)` assertions are safe because a false positive still locates the token (it just also matches the longer form), but the negative direction silently passes when it should fail.

### Assert Lucide icons by specific class, not the generic `/lucide/` class

Every Lucide icon receives a class like `lucide-settings`, `lucide-plus`, `lucide-rotate-ccw`, etc., in addition to the
generic `lucide` class that all icons share. When asserting that a specific icon is rendered, match against the
specific class (e.g. `/lucide-settings/`) rather than `/lucide/` — otherwise any Lucide icon satisfies the assertion and
an accidental icon swap is undetected:

```ts
// WRONG — passes for any Lucide icon, not just the settings icon
expect(screen.getByRole("button", { ... })).toContainHTML("lucide");

// CORRECT — fails if the icon is swapped for a different one
const icon = container.querySelector(".lucide-settings");
expect(icon).toBeInTheDocument();
```

Note: the specific `lucide-<name>` class is a library implementation detail — it can change if lucide-react renames its class scheme in a future major version, or if the icon is replaced with an equivalent one from another source. When the test goal is the **accessibility contract** (the icon must be decorative and hidden from assistive technology), assert the behavioral attribute instead:

```ts
// Behavioral: verifies the icon is hidden from screen readers regardless of which icon is used
expect(container.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
```

Reserve the specific-class assertion for cases where the exact icon identity matters for visual regression, and prefer a Playwright screenshot or Storybook snapshot for those cases.

### Test keyboard tab order with `element.focus()` + single `user.tab()`, not DOM position

`compareDocumentPosition` checks DOM tree position, which is not the same as keyboard tab order. A `tabindex` attribute can reverse tab order without changing DOM order, so a `compareDocumentPosition` assertion can pass while keyboard navigation is broken.

Two patterns to avoid:

**Pattern A — DOM position check (wrong):**
```ts
expect(
  addLink.compareDocumentPosition(menuButton) & Node.DOCUMENT_POSITION_FOLLOWING,
).toBeTruthy();
```
Passes even when `tabindex="-1"` on `addLink` makes it unreachable by keyboard.

**Pattern B — counting tab stops (fragile):**
```ts
await user.tab(); // Focus: logo link
await user.tab(); // Focus: '+' link
expect(addLink).toHaveFocus();
```
Encodes the count of preceding focusable elements. A future change that inserts one focusable element before `addLink` silently breaks the assertion with no message pointing at the added element.

**Correct pattern — anchor focus, then tab once:**
```ts
addLink.focus();           // anchor: no dependency on preceding elements
expect(addLink).toHaveFocus();
await user.tab();          // next tab stop
expect(menuButton).toHaveFocus();
```
This tests the behavioral contract (tab from `addLink` lands on `menuButton`) without pinning how many other elements exist beforehand.

Use `await user.tab()` from a fixed anchor any time you need to assert that one interactive element is reachable immediately after another in keyboard navigation order.

`compareDocumentPosition(other) & Node.DOCUMENT_POSITION_FOLLOWING` is wrong for keyboard tab order — for exactly the reason above — but it is the correct, intended tool for asserting **visual left/right layout order** in a `flex justify-between` two-slot row. jsdom has no layout engine, so DOM source order is the right proxy for visual position in a standard LTR flex container where slots are fixed in source order. The assertion is wrong only for keyboard navigation (because `tabindex` decouples tab order from DOM order) and would also not catch a CSS-only `flex-row-reverse` regression (which jsdom cannot compute). For DOM-order-as-visual-order checks, the pattern is sound. Reference: the `describe("BatchImportWizard footer layout")` block in `frontend/src/components/batch-import/batch-import-wizard.test.tsx` uses `compareDocumentPosition` to pin Cancel→primary, Back→Import, and Back→Done slot order in the `WizardFooter` component.

### Pin specific `indexOf` positions in DOM-order tests

`expect(brandIndex).toBeGreaterThan(0)` is ambiguous in both failure directions: it passes when `brandIndex` is `2` (wrong order relative to form) and fails with a numeric mismatch when `brandIndex` is `-1` (element absent), with no message naming the missing element. Pin both positions explicitly and add a separate `-1` guard so each failure names its own problem:

```ts
// BEFORE — loose: passes at index 2 (wrong order), fails cryptically at -1
expect(brandIndex).toBeGreaterThan(0);

// AFTER — pinned: fails with exact value mismatch; -1 guard names the absent element
expect(brandIndex).not.toBe(-1);  // guard: names the element if absent
expect(formIndex).toBe(0);        // pinned position
expect(brandIndex).toBe(1);       // pinned position
```

Apply this pattern whenever `children.indexOf(el)` or `findIndex(...)` is used to assert DOM order. The `not.toBe(-1)` guard must come before the positional assertion; otherwise the positional failure message shows `-1 !== 1` and does not indicate which element is missing. Reference: `frontend/src/app/login/page.test.tsx` lines 169–175.

### Assert `disabled` state behaviorally, not just by attribute

`.toBeDisabled()` verifies the HTML `disabled` attribute but does not verify that a click is actually ignored.
The behavioral contract is: "a click does not invoke the callback." Add `await user.click(btn)` followed by
`expect(callback).not.toHaveBeenCalled()` so a future refactor from `<button disabled>` to a non-`<button>` element
with a custom click handler is caught:

```ts
expect(btn).toBeDisabled();             // attribute check

await user.click(btn);
expect(onRate).not.toHaveBeenCalled();  // behavioral check
```

Reference: `frontend/src/components/learn/learn-action-bar.test.tsx` — the disabled test asserts both `.toBeDisabled()`
and that `onRate` is not called after clicking all three buttons.

### Add `vi.mock` for every new child component at the top of the test file

`vi.mock` calls are hoisted to the top of the module by Vite before any import executes. This means mock factory functions run before the module graph resolves — but it also means a `vi.mock` added inside a `describe` block or after the first import silently has no effect if the call is not physically at the top of the file.

When a component under test gains a new child component (e.g. `LearnClient` now renders `LearnActionBar`), the mock for that child **must be added at the top of the test file alongside the existing `vi.mock` calls**, not inside `beforeEach` or a nested `describe`. Missing this produces a jsdom environment crash or a partial render that silently omits the child's DOM output, making state-assertion tests appear to pass on no-op renders.

The pattern for a child component that has a stateful prop the integration test needs to inspect:

```ts
vi.mock("@/components/learn/learn-action-bar", () => ({
  LearnActionBar: (props: { onRate: (d: "left" | "down" | "right") => void; disabled: boolean }) => (
    <div data-testid="learn-action-bar" data-disabled={String(props.disabled)}>
      <button type="button" onClick={() => props.onRate("left")} disabled={props.disabled}>
        Rate as Again
      </button>
      {/* ... */}
    </div>
  ),
}));
```

The `data-disabled={String(props.disabled)}` attribute exposes the structural disabled state without relying on CSS classes or the HTML `disabled` attribute — both of which may be absent when the disabled state is implemented via pointer-events or opacity only. Assertions then read:

```ts
expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "false");
expect(screen.getByTestId("learn-action-bar")).toHaveAttribute("data-disabled", "true");
```

Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` — `LearnActionBar` mock at the top alongside the `SwipeCardStack` mock.

### Grep `frontend/` not `frontend/src/` when deleting a component

A plan that says "delete component `X`" must locate every reference to `X` across the **whole** `frontend/` tree, not just `frontend/src/`. Tests live in two locations: co-located under `frontend/src/**/<file>.test.tsx` AND top-level under `frontend/__tests__/**/*.test.tsx` (see [the narrow / broad split contract](#the-narrow--broad-split-is-a-contract)). A `grep -r X frontend/src/` reports zero hits while a stale top-level test still imports the deleted component, the file fails at run time, and CI catches the gap only after the cleanup commit lands.

```bash
# AVOID: misses frontend/__tests__/, frontend/e2e/, frontend/playwright.config.ts.
grep -rn "ModeBadge" frontend/src/

# PREFER: catches every committed reference.
grep -rn "ModeBadge" frontend/ --include="*.ts" --include="*.tsx"
```

**Why:** the top-level `frontend/__tests__/` directory holds broad page tests by convention, and an older feature's broad test may import a presentational component that the current refactor is removing. The Biome lint passes on a stale test file (it just imports a now-undefined symbol from a path that still exists), and the runtime failure does not surface until `pnpm test` runs. Since CI already runs the full test suite, the failure surfaces eventually — but a tighter grep up-front avoids the cleanup-commit-then-fix-tests churn.

**How to apply:** before `git rm`-ing a component file, grep the whole `frontend/` directory (not just `frontend/src/`). The same rule extends to any cross-tree symbol: the GraphQL `graphql()` document discovery scans `frontend/src/**` but Playwright specs under `frontend/e2e/` reference page paths and component selectors, and `frontend/playwright.config.ts` may reference paths that contain the removed component name in a comment or fixture.

**Coverage migration is a separate audit step from import cleanup.** When a component is being deleted because its responsibilities have moved to a replacement (e.g. a nav component split across `AppShell`, `GlobalRail`, and `LogoDrawer`), the `<deleted>.test.tsx` file is usually deleted alongside the component — but the assertions inside that test file may have been the only coverage for behaviour that now lives in a different component or in the parent layout. Deleting the test file without re-homing those assertions silently drops coverage for branches the replacement implementation can also fail. Before deleting `<deleted>.test.tsx`:

1. List every `it(...)` / `test(...)` inside it.
2. For each, decide whether the asserted behaviour still exists somewhere (in the replacement component, in the layout that hosts it, in a route-level test). If yes, ensure a new test in that location covers the same branch.
3. For each behaviour that no longer exists, the deletion is correct — but say so in the commit message so reviewers can verify intent rather than guess.

**Worked example: header auth degradation branches.** When the header auth logic moved into `app/layout.tsx`, the old `global-header.test.tsx` suite was deleted alongside the deleted component — but eight branches asserting that the root layout degrades silently on `getUser()` failure / `gqlFetch` `UNAUTHENTICATED` / `me`-fetch failure went with it. Those branches still exist in the replacement implementation and can still fail, so the audit step is to re-home them. The fix was `app/layout.test.tsx` (`frontend/src/app/layout.test.tsx`), which re-asserts every branch against the post-refactor implementation. Without that re-homing the branches would have stayed silently uncovered until a regression surfaced in production.

### Fake timer hygiene: use `beforeEach` / `afterEach` for setup and teardown

Inline calls to `vi.useFakeTimers()` inside a single `it(...)` block are leak-prone. If that test throws before reaching `vi.useRealTimers()`, every subsequent test in the file inherits the fake clock — `userEvent` interactions hang (their internal `setTimeout(0)` never fires), `MockedProvider` async resolution stalls, and the suite times out with no stack pointing at the offending test.

The safer pattern installs and tears down fake timers in `beforeEach` / `afterEach` so cleanup is unconditional:

```ts
beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
  // Place vi.useRealTimers() before other teardown so subsequent afterEach
  // hooks (e.g. restoreAllMocks) run on real time.
});
```

This applies to any file that touches fake timers — or *might* touch them after a refactor. The `afterEach` guard is cheap to add preemptively and prevents silent time-pollution across the whole suite.

Note: `vi.useFakeTimers()` / `vi.useRealTimers()` is complementary to — not a replacement for — the `vi.spyOn` + `vi.restoreAllMocks()` pair documented in the [`vi.spyOn` section above](#vispyon-requires-virestoreallmocks-in-aftereach). Run both pairs when a file uses both spies and timers.

### Testing a 5-second undo-toast window: `shouldAdvanceTime` + `advanceTimers`

`UndoDeleteProvider` schedules the `commitDelete` call 5 000 ms after `scheduleDelete` fires. Testing the timer elapse requires fake timers, but `userEvent` relies on its own internal `setTimeout(0)` calls to flush pointer-event queues. A plain `vi.useFakeTimers()` freezes those internal timers and hangs any `await user.click(...)` before the undo window can be advanced.

The correct combination is `shouldAdvanceTime: true` (lets real-time progress drive fake-timer advancement, so `userEvent`'s internal delays still resolve) paired with `userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) })` (bridges userEvent's internal scheduler to Vitest's fake clock):

```ts
beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
});
afterEach(() => {
  vi.useRealTimers();
});

it("commits delete after the undo window", async () => {
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  // ... render, click delete button ...
  vi.advanceTimersByTime(5100); // advance past the 5 s window
  await waitFor(() => {
    expect(deleteMock.result).toHaveBeenCalled();
  });
});
```

Call `vi.advanceTimersByTime(5100)` (100 ms buffer beyond the 5 000 ms window) after the user interaction settles; then use `waitFor` to let the mock consume the mutation response asynchronously.

### `UndoDeleteProvider` wrapping is required for components that call `useUndoDelete()`

A component that calls `useUndoDelete()` reads from `UndoDeleteContext`. Any test that renders the component without `<UndoDeleteProvider>` around it throws at context initialisation — not at the assertion — which produces a misleading "context is undefined" error that points at `undo-delete.tsx` rather than the test's missing wrapper.

Additionally, `UndoDeleteProvider` calls `usePathname()` internally (to flush pending deletes on navigation). Tests that wrap with `UndoDeleteProvider` must also mock `next/navigation`:

```ts
vi.mock("next/navigation", () => ({
  usePathname: () => "/admin/roles",
}));
```

Include the mock at the top of the test file, before any `beforeEach` or `describe` blocks, so the mock is in place when `UndoDeleteProvider` mounts during the first `render(...)` call.

### ref-as-prop mock strategy for React 19 components

React 19 adopts the `ref`-as-prop pattern: components that expose an imperative handle declare `ref?: RefObject<H | null>` in their props interface rather than using `forwardRef`. Mocking such a component in Vitest requires `useImperativeHandle` inside the mock factory so the test can exercise the handle's methods:

```ts
import { useImperativeHandle } from "react";
import type { RefObject } from "react";

vi.mock("@/components/learn/swipe-card-stack", () => ({
  default: vi.fn(
    (props: { ref?: RefObject<SwipeCardStackHandle | null>; /* other props */ }) => {
      useImperativeHandle(props.ref, () => ({
        triggerSwipe: triggerSwipeSpy,
      }));
      return <div data-testid="swipe-card-stack-stub" />;
    },
  ),
}));
```

In the test body, create the ref with `createRef<H>()` from `@testing-library/react` (re-exported from React) and pass it as a prop:

```ts
import { createRef } from "react";

it("calls triggerSwipe on the handle", async () => {
  const ref = createRef<SwipeCardStackHandle>();
  render(<ParentComponent swipeRef={ref} />);
  // Interact with parent, then assert on the handle
  expect(triggerSwipeSpy).toHaveBeenCalledWith("right");
});
```

Key constraints:

- `useImperativeHandle` must be called unconditionally inside the mock factory function — calling it conditionally (e.g. `if (props.ref)`) violates React's rules of hooks and causes a test-environment warning.
- The `spy` variable (`triggerSwipeSpy` above) must be declared at module scope so it is stable across renders and accessible in assertions.
- The mock factory's `props` type annotation must include `ref?: RefObject<H | null>` explicitly; without it TypeScript infers `props` as `{}` and the `useImperativeHandle` call cannot reference `props.ref`.

Reference: `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` mocking `swipe-card-stack` with a `triggerSwipe` spy exposed via `useImperativeHandle`.

### `queryBy*` for absence assertions, `getBy*` for presence assertions

`getBy*` queries throw immediately when the element is not found, which makes them unsuitable for asserting that an element is absent — the thrown error kills the test before the `.not.toBeInTheDocument()` assertion can run. `queryBy*` returns `null` on a miss and lets `.not.toBeInTheDocument()` execute correctly.

Rule:
- Asserting **absence**: always use `queryBy*`. Example: `expect(screen.queryByRole("button", { name: /delete admin/i })).toBeNull()`.
- Asserting **presence**: always use `getBy*`. Example: `expect(screen.getByText("System role")).toBeInTheDocument()`.

`findBy*` is the async variant of `getBy*` — use it when the element appears after an async operation (query resolution, timer, animation), not for static renders.

Never use `getBy*` with a `.not.toBeInTheDocument()` assertion: the test will always fail with a "not found" error rather than a clean assertion failure, hiding the real intent.

### Assert `role="alert"` absence on clean render paths

An always-rendered `<p role="alert">` wrapper element — one whose text content is conditionally set but whose DOM node is unconditionally present — passes a `queryByText` or `queryByRole("alert", { name: /.../ })` assertion on the clean path (no matching text) while silently hiding the structural regression that the alert node is never removed. The `queryByRole("alert").not.toBeInTheDocument()` assertion catches that regression: if the alert element exists regardless of error state, the assertion fails immediately.

On the clean render path (no error), assert the element is absent entirely. Pair this with a positive assertion on the error path so both sides of the conditional are covered:

```ts
// clean path — no error param
expect(screen.queryByRole("alert")).not.toBeInTheDocument();

// error path — error param present
expect(screen.getByRole("alert")).toHaveTextContent(/sign-in failed/i);
```

Reference: `frontend/src/app/login/page.test.tsx` lines 209–215 (absence) and 200–206 (presence).

### Expand a Radix `Collapsible` before asserting its inner content

Radix `CollapsibleContent` is `Presence`-based: when collapsed, its children are **unmounted** (removed from the DOM), not merely visually hidden. While collapsed, `screen.queryByText("apple")` returns `null` — the node does not exist. To assert the content, click the trigger first, then assert:

```ts
// Before clicking: content is absent — queryByText returns null.
expect(screen.queryByText("apple")).not.toBeInTheDocument();

// Click the trigger to expand.
await user.click(screen.getByRole("button", { name: /show preview \(2\)/i }));

// After expanding: content is mounted and queryable.
await waitFor(() => {
  expect(screen.getByText("apple")).toBeInTheDocument();
});
```

To assert that content is hidden while collapsed, assert `not.toBeInTheDocument()` (absence), not `not.toBeVisible()` — the element is unmounted, so visibility-based assertions will throw "element not found" rather than failing cleanly.

In Playwright, click the trigger before asserting cells: `await page.getByRole("button", { name: /Show preview/ }).click()`, then `await expect(page.getByRole("cell", { name: frontA })).toBeVisible()`.

Worked example: `frontend/src/components/batch-import/batch-import-wizard.test.tsx` (valid-validate test, lines 188–200) asserts `queryByText("apple")` is `null` before clicking the "Show preview (2)" trigger and present after. `frontend/e2e/cardgroup-import.spec.ts` (lines 69–71) clicks the trigger before asserting cells.

