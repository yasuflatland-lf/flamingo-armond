# Testing convention: narrow vs broad page tests

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

All tests live under `frontend/__tests__/` using Vitest + Testing Library. Two naming conventions split responsibility:

**Narrow tests** (`<feature>-<flow>.test.tsx`) isolate a single user-facing flow introduced by a feature PR. Examples: `cards-pagination.test.tsx` (pagination + fetchMore only), `cards-bulk-delete.test.tsx` (selection and delete only), `admin-dictionary-import.test.tsx` (validate-then-import flow), `admin-users-roles.test.tsx` (assign/revoke roles only), `admin-roles-crud.test.tsx` (create/update/delete only), `admin-layout.test.tsx` (admin gate only). Each narrow test is shipped by the feature PR that introduced its flow, locking in expected behaviour.

**Broad tests** (`<page>.test.tsx`) guard the page-level composition and integration points across PRs. Examples: `cardgroups-list.test.tsx`, `cardgroups-detail.test.tsx`, `cards-list.test.tsx`, `admin-users.test.tsx`, `admin-roles.test.tsx`, `admin-dictionary.test.tsx`. Each broad test covers SSR auth gate, initial render, empty state, and error boundaries — without duplicating the narrow test's flow-specific assertions.

**Anti-pattern**: Do not name a flow-specific test with a page-level name. If a feature PR introduces a flow that is the only flow on its page, still name the test `<page>-<flow>.test.tsx` to reserve the `<page>.test.tsx` slot for the future broad test.

**Shared utilities** live under `frontend/__tests__/utils/` and `frontend/__tests__/fixtures/`:

- `mock-supabase.ts` — in-memory `getUser` mock for Supabase server client in RSC tests.
- `mock-apollo-paginated.ts` — one-mock-per-fetchMore helper with inline documentation. Provides `installApolloMockLeakSpy`, which captures `console.warn` calls matching `"No more mocked responses for the query"`; calling `assertNoLeaks()` in `afterEach` throws if any were recorded, catching double-fetch regressions.
- `fixtures/users.ts` and `fixtures/cardgroups.ts` — shared test data.

**JSDoc on shared utilities is part of the contract.** Reviewers should treat shared helpers under `__tests__/utils/` as if a new contributor will copy their usage examples verbatim — runnable copy-paste-ready snippets, not approximations. Document which fields each state-mutating knob touches (e.g. a `setError` that does not clear a previously-set user) so chained calls have predictable observed behaviour.

### The narrow / broad split is a contract

Putting a flow-detail assertion in a broad-named file (e.g. a role-checkbox toggle inside `admin-users.test.tsx`) silently locks in implementation detail and forces the broad test to break on every refactor of the narrow flow. The narrow / broad split is not a guideline — it is a contract: broad tests assert only page-level composition (SSR auth gate, initial render, empty state, error boundaries); flow-specific assertions belong in their narrow companion file.

A page can host **both** a co-located `<page>.test.tsx` (next to the source under `src/app/...`) and a `__tests__/<page>.test.tsx` (broad scope) file. The co-located test focuses on the page's local refactor surface (e.g. stubbing the client component); the `__tests__/` file mounts the full tree end-to-end. Coverage between the two MUST be deconflicted manually — the author of any new broad test must read both before adding assertions, otherwise duplicate redirect / auth-gate cases accumulate across the two files.

### RSC test rendering pattern

Tests for Next.js 15+ async server components render by `await`ing the page function and passing its element tree to `render(...)`. The page params argument is `Promise<{...}>`, not a plain object:

```ts
render(await CardgroupDetailPage({ params: Promise.resolve({ id: "cg-1" }) }));
```

Pre-15 patterns that pass `{ params: { id } }` directly will not type-check or will misbehave at runtime.

`createSupabaseServerClient` is server-only, so RSC tests must stub it. The repo has no MSW; the canonical pattern is a per-test `vi.mock("@/lib/supabase/server", ...)` factory backed by the shared `mockSupabaseServerClient()` helper, with per-case `setMockSupabaseUser(...)` calls in `beforeEach`. The `server-only` import is also stubbed at the Vitest config level (`vitest.config.ts`) so any module that pulls it in transitively does not crash the test runner.

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

Reference: `frontend/src/components/nav/global-header.test.tsx`. This applies to any test file that calls `vi.spyOn(...)` on a global (`console`, `Date`, `crypto`) or a module export.

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
// frontend/src/components/nav/fab-action.ts — pure helper, no React, no next/navigation imports.
export function resolveFabAction(pathname: string): FabAction | null { /* ... */ }

// frontend/src/components/nav/fab-action.test.ts
// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveFabAction } from "./fab-action";

describe("resolveFabAction", () => {
  it("returns Add new cardgroup href for exact /cardgroups", () => {
    expect(resolveFabAction("/cardgroups")).toEqual({ kind: "cardgroup", /* ... */ });
  });
});
```

The component then becomes a thin shell that calls the helper:

```tsx
// frontend/src/components/nav/global-fab.tsx
const action = resolveFabAction(pathname);
if (action === null) return null;
```

**Why:** pure logic + jsdom is wasted overhead — every test pays for the DOM environment to assert a string-in-string-out result. Node tests are faster (no jsdom bootstrap), clearer (no `vi.mock` of `next/navigation`), and the production code gets a forcing function to keep the helper React-free. The same testability-extraction principle is documented for the backend in [`docs/backend/library-gotchas/extract-startup-helpers-for-branch-coverage.md`](../backend/library-gotchas/extract-startup-helpers-for-branch-coverage.md).

**How to apply:** when a client component's render function or hook callback contains branching logic that depends only on its arguments (not on React state, refs, or router objects), lift that logic into a sibling `.ts` file with no React or Next.js imports, and write its tests under `// @vitest-environment node`. The component imports the helper and calls it. Reference: `frontend/src/components/nav/fab-action.ts` consumed by `global-fab.tsx` and `header-add-card-link.tsx`; the test file `fab-action.test.ts` runs under the node environment while the component tests stay on jsdom.

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

Exception: class names that control behavior-affecting DOM attributes (e.g., `pointer-events-none` as a layout mechanism) are still unsuitable for jsdom assertions because jsdom does not enforce hit-testing. Document those invariants in a comment on the component instead.

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

### Mobile and PC entry points for the same action must produce matching hrefs

When a feature has two surfaces that construct the same href (e.g. a mobile in-header button that decodes
`usePathname()` and a desktop floating button that receives the already-decoded id as a prop), assert that the
constructed URLs are identical for the same logical input. The mismatch is invisible for plain alphanumeric UUIDs
but real for any special-character-bearing id:

```ts
// mobile path: decodes usePathname() → re-encodes at href construction
const mobileHref = `/cards/new?cardgroup=${encodeURIComponent(rawId)}&return=/learn/${encodeURIComponent(rawId)}`;

// desktop path: receives decoded prop → encodes at href construction
const encodedId = encodeURIComponent(cardgroupId);
const desktopHref = `/cards/new?cardgroup=${encodedId}&return=/learn/${encodedId}`;

expect(desktopHref).toBe(mobileHref);
```

Reference: `frontend/src/components/nav/logo-drawer.tsx` (mobile `+` link, decodes from `usePathname()`) and
`frontend/src/components/nav/learn-add-card-floating.tsx` (PC ghost icon, receives decoded `cardgroupId` prop).

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

**Coverage migration is a separate audit step from import cleanup.** When a component is being deleted because its responsibilities have moved to a replacement (e.g. `GlobalHeader` → `AppShell` + `GlobalRail` + `LogoDrawer`), the `<deleted>.test.tsx` file is usually deleted alongside the component — but the assertions inside that test file may have been the only coverage for behaviour that now lives in a different component or in the parent layout. Deleting the test file without re-homing those assertions silently drops coverage for branches the replacement implementation can also fail. Before deleting `<deleted>.test.tsx`:

1. List every `it(...)` / `test(...)` inside it.
2. For each, decide whether the asserted behaviour still exists somewhere (in the replacement component, in the layout that hosts it, in a route-level test). If yes, ensure a new test in that location covers the same branch.
3. For each behaviour that no longer exists, the deletion is correct — but say so in the commit message so reviewers can verify intent rather than guess.

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

