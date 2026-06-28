# Auth check runs outside the Suspense boundary — data fetch runs inside

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

When a route uses `loading.tsx` + `<Suspense>` to stream the shell before the data fetch resolves, auth validation must run in the outer RSC (`page.tsx`) before the `<Suspense>` element is returned. If the redirect runs from within the suspended subtree, the skeleton fallback flashes to the user before the navigation fires — because Next.js streams the fallback immediately while the suspended component awaits its async work.

The structural split is:

```tsx
// frontend/src/app/cardgroups/page.tsx
import { headers } from "next/headers";
import { readAuthContext } from "@/lib/supabase/auth-status";

export default async function CardgroupsPage() {
  // Auth runs OUTSIDE the Suspense boundary so an unauthenticated request
  // redirects to /login before any streaming starts.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/login");

  return (
    <Suspense fallback={<CardgroupsSkeleton />}>
      <CardgroupsContent />     {/* GraphQL fetch lives here */}
    </Suspense>
  );
}

export async function CardgroupsContent() {
  // gqlFetch happens inside the boundary. A second UNAUTHENTICATED check
  // is still required — the middleware-forwarded status confirms the Supabase
  // session, but the GraphQL JWT may be independently revoked (clock skew,
  // JWKS rotation, short-lived edge token).
  try {
    const data = await gqlFetch(MyCardgroupsConnectionQuery, { revalidate: 0 });
    return <CardgroupsClient initialConnection={data.myCardgroupsConnection} />;
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error("[cardgroups] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }
}
```

**Why the inner component still needs its own auth check:** the outer gate validates the middleware-forwarded session status. The inner `gqlFetch` validates the GraphQL JWT issued from that session. Clock skew, JWKS rotation, or a short-lived edge token can make the session status valid while the backend rejects the JWT. If the inner component omits the `isUnauthenticatedGraphQLError` check, a revoked JWT throws into the `<Suspense>` boundary's nearest `error.tsx`, showing an error page rather than redirecting to `/login`.

## Named export `CardgroupsContent` for testability

The inner async server component is exported as a named export — not the page default — so the RSC test can call it directly. React's Suspense renderer cannot resolve a suspended subtree synchronously inside Vitest; exporting `CardgroupsContent` separately lets the test render the data layer in isolation while a sibling test verifies the outer `<Suspense>` boundary and skeleton fallback:

```ts
// In the test file
import CardgroupsPage, { CardgroupsContent } from "./page";

// Outer shell test — asserts Suspense structure without resolving gqlFetch
it("returns a <Suspense> boundary with <CardgroupsSkeleton /> as fallback", async () => {
  const jsx = await CardgroupsPage();
  expect(jsx.type).toBe(Suspense);
  expect(jsx.props.fallback.type).toBe(CardgroupsSkeleton);
  expect(jsx.props.children.type).toBe(CardgroupsContent);
});

// Inner content test — invokes CardgroupsContent directly, gqlFetch is mocked
it("passes connection edges to CardgroupsClient", async () => {
  vi.mocked(gqlFetch).mockResolvedValue(makeConnection([...]) as never);
  const jsx = await CardgroupsContent();
  render(jsx);
  expect(screen.getByText("Spanish Vocab")).toBeInTheDocument();
});
```

Apply this split to every page that uses `loading.tsx` + `<Suspense>`: outer `page.tsx` handles auth and returns the Suspense shell; inner `*Content` handles data and is named-exported for direct test invocation. The `loading.tsx` file renders the skeleton that Next.js displays automatically during navigation to the route, so the `<Suspense fallback={...}>` covers in-page streaming while `loading.tsx` covers the initial route transition.

Reference: `frontend/src/app/cardgroups/page.tsx`, `frontend/src/app/cardgroups/page.test.tsx`, `frontend/src/app/cards/new/page.tsx`, `frontend/src/app/learn/[cardgroupId]/page.tsx`. Each outer `page.tsx` gates on `readAuthContext(await headers()).status !== "authenticated"`; each inner `*Content` component checks `isUnauthenticatedGraphQLError` for the independent JWT path.

## Contrast: display-hint resolution that degrades (runs inside Suspense)

The rule above applies specifically to auth *gates* that must `redirect()` on failure. A component whose auth failure mode is degradation rather than redirection may safely run inside a `<Suspense>` boundary — provided it wraps its awaits in `try/catch` so transport rejections do not escape.

`ConditionalShell` renders `AppShell` (`frontend/src/components/conditional-shell.tsx` → `frontend/src/components/nav/app-shell.tsx`) as a **synchronous prop-driven wrapper** (`{ user, isAdmin, children }`) that performs no auth I/O and therefore never suspends. Identity is resolved upstream by the middleware (`frontend/src/lib/supabase/middleware.ts`), which calls `getClaims()` (wrapped in try/catch) and forwards the result via request headers; `app/layout.tsx` reads them via `readAuthContext` and passes the resulting `user` and `isAdmin` props down through `ConditionalShell` to `AppShell`. The shell is synchronous, so no Suspense boundary wraps it.

The abstract rule — that a component whose failure mode is degradation rather than redirection may safely run inside a `<Suspense>` boundary — remains valid for other async RSC patterns. The synchronous shell is not an instance of it: it performs no async work to suspend on.

The distinguishing question is: **what happens on failure?**

- Failure → `redirect()`: run **outside** Suspense (a redirect from inside a suspended subtree flashes the fallback before navigating).
- Failure → degrade to safe default: may run **inside** Suspense, but the component must `try/catch` all async work so transport errors degrade rather than escape.

See [`suspense-does-not-catch-thrown-errors.md`](suspense-does-not-catch-thrown-errors.md) for the full explanation of why `<Suspense>` does not catch thrown errors and the try/catch pattern required for components that run inside the boundary.
