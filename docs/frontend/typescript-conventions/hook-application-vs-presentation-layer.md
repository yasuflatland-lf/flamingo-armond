# Custom hook contract: Application-layer state, Presentation-layer composition

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A custom hook that bakes a UX choice into its return value (e.g. "this boolean is true when the spinner should show") makes the hook unusable for any consumer that wants a different spinner policy. The cleaner split is to expose the **raw Application-layer state** (`loading`, `networkStatus`, `fetchingMore`, `edges`) and let each consumer compose the boolean it actually needs for its own UX.

The failure mode is subtle: a hook field named `fetchingMore` looks like it means "another page is being fetched", but if the hook defines it as `networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0)`, the field also returns `true` during the initial load of a route that was seeded from SSR. The OR clause is Presentation logic ("show a spinner ALSO during initial-load-with-seed") leaking into the Application-layer contract. The next consumer of the hook inherits that policy whether they want it or not.

```ts
// Anti-pattern: hook bakes a UX gate into the returned boolean.
// "fetchingMore" no longer means "fetchMore is in flight" — it means
// "the spinner should show, per this one consumer's UX policy".
export function useCardsConnection(input: UseCardsConnectionInput): UseCardsConnectionResult {
  // ...
  const fetchingMore =
    networkStatus === NetworkStatus.fetchMore || (loading && edges.length > 0);
  return { edges, fetchingMore /* loading is hidden */ };
}

// In the consumer:
{!fetchMoreError && fetchingMore && pageInfo.hasNextPage && <SpinnerCopy />}
// Looks innocent — but any new consumer (a learn page, an admin list) inherits
// the OR clause whether or not their UX wants it.
```

```ts
// Correct: hook exposes one meaning per field. Consumer composes the gate.
export function useCardsConnection(input: UseCardsConnectionInput): UseCardsConnectionResult {
  // ...
  const fetchingMore = networkStatus === NetworkStatus.fetchMore; // pure
  return { edges, loading, fetchingMore /* both exposed */ };
}

// In the consumer — the OR clause lives at the call site.
{!fetchMoreError &&
  (fetchingMore || (loading && edges.length > 0)) &&
  pageInfo.hasNextPage && <SpinnerCopy />}
```

**Why:** the hook is an Application-layer adapter over Apollo's `useQuery`. Its job is to expose the connection state in a form the route can read; deciding when a spinner appears is a Presentation-layer concern that depends on the route's information architecture (does this page have an SSR seed? does it show a skeleton? does the empty state count as "loaded"?). A future consumer — a learn page that wants no spinner during initial load because it shows a card-deck skeleton instead — would have to either fork the hook or work around the baked-in OR clause. Both outcomes are worse than letting each consumer write its own one-line composition.

Single-Responsibility Principle on a hook field: each returned value has one meaning that does not depend on the consumer. `fetchingMore` means "is `fetchMore` in flight"; `loading` means "is the underlying `useQuery` in any kind of loading state"; the consumer combines them as its UX requires.

**How to apply:** when adding a derived boolean to a hook's return type, ask "is this derivation valid for every plausible consumer, or only the one I'm building today?". If only today's consumer benefits, push the derivation back to the consumer and expose the raw operands instead. The hook should grow new fields only when the new field has one consumer-independent meaning. Reference: `useCardsConnection` in `frontend/src/app/cardgroups/[id]/cards/use-cards-connection.ts` returns `fetchingMore = networkStatus === NetworkStatus.fetchMore` and `loading` separately; `CardsClient` in `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` composes the spinner gate as `(fetchingMore || (loading && edges.length > 0))`.

This is the frontend analog of the backend Application-vs-Presentation split codified in [`.claude/rules/backend-layering.md`](../../../.claude/rules/backend-layering.md): the usecase layer returns typed values that are agnostic to the GraphQL wire format, and the resolver composes the wire-format response. The custom-hook layer is structurally identical — return raw operands, let the React consumer compose the UX.
