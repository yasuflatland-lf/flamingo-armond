# Initial-load UNAUTHENTICATED redirects belong in the RSC `page.tsx`, not in the client `error.tsx`

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A client `error.tsx` boundary that performs `router.replace("/login")` from a `useEffect` keyed on `error.message.includes("UNAUTHENTICATED")` is a band-aid: the boundary already received the error, the page rendered the boundary's fallback shell once, and the redirect fires after a post-render microtask. The user briefly sees the "Couldn't load your profile" copy before being navigated. More importantly, substring-matching `error.message` for routing decisions is the exact pattern [§ "Structurally parse GraphQL `extensions.code`"](../../../.claude/rules/frontend-rsc-error-handling.md#structurally-parse-graphql-extensionscode--never-substring-match-the-message) forbids — it conflates a real `UNAUTHENTICATED` extension with any error whose message text happens to contain the word.

The fix is to intercept the GraphQL error in the RSC `page.tsx` itself, before it bubbles into the boundary, using the structural helper:

```tsx
// frontend/src/app/profile/page.tsx
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";

export default async function ProfilePage() {
  const supabase = await createSupabaseServerClient();
  const { data: { user }, error: authErr } = await supabase.auth.getUser();
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[profile] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  let data: MeQueryType;
  try {
    data = await gqlFetch(MeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[profile] gqlFetch failed:", err);
    throw err;
  }
  /* ...render with data... */
}
```

The `error.tsx` boundary then handles only the residual failure modes (network 5xx, infrastructure errors, mid-session UNAUTHENTICATED from client-side mutations after hydration). It logs the error with `digest` and renders a Retry button — no `router.replace`, no message-substring branching:

```tsx
// frontend/src/app/profile/error.tsx
"use client";

export default function ProfileError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    // Initial-load UNAUTHENTICATED is intercepted in page.tsx and redirects to
    // /login before this boundary is reached. This boundary handles residual
    // failure modes (network, 5xx, post-hydration UNAUTHENTICATED).
    console.error("[/profile error boundary]", { message: error.message, digest: error.digest });
  }, [error]);
  return (/* ...heading + Retry button... */);
}
```

**Why:** the RSC has the GraphQL error in scope at the `try/catch` boundary BEFORE the client component ever renders, so the `redirect()` happens server-side and the user navigates without seeing the error fallback. Moving the redirect to the RSC also removes the substring-match dependency: `isUnauthenticatedGraphQLError` parses the structured extension, so the routing decision is correct even when the upstream message text changes. Pin the structural log assertion in the boundary's test — `expect.objectContaining({ message: "...", digest: "..." })` — using `digest` as the discriminating key (`Error.prototype` does not have `digest`) per [`docs/frontend/typescript-conventions.md` § "`expect.objectContaining({ message })` is not enough — add a discriminating key"](../typescript-conventions/expect-objectcontaining-message-is-not-enough.md).

**How to apply:** any RSC `page.tsx` whose data fetch can return GraphQL `UNAUTHENTICATED` (i.e. any auth-gated query) MUST intercept the error and `redirect("/login")` from the page itself, using `isUnauthenticatedGraphQLError`. The sibling `error.tsx` keeps a small log-and-retry shell for residual failure modes; do not put `router.replace` in `error.tsx`. Today's pattern: `frontend/src/app/profile/page.tsx` (RSC redirect) + `frontend/src/app/profile/error.tsx` (degraded boundary). This rule pairs with [§ "Structurally parse GraphQL `extensions.code`"](../../../.claude/rules/frontend-rsc-error-handling.md#structurally-parse-graphql-extensionscode--never-substring-match-the-message) — both eliminate substring-matching on error.message at the routing seam.
