# Empty result is a business state, not an error

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A GraphQL query that returns an empty array is a **valid, successful response**. Treating it as an error condition — by throwing, redirecting, or activating an error boundary — conflates two orthogonal concepts: transport/server failure vs. a domain state where the result set happens to be empty.

The canonical example is `learnNextDueCards`: when all cards in a cardgroup have a `due` timestamp in the future, the resolver returns `[]`. This is the "all caught up" state — every card has been reviewed for today. It is not a network error, not a GraphQL error, and not an exceptional condition.

```ts
// page.tsx — RSC server component
const cards = cardsData.learnNextDueCards; // [] when all caught up — that is correct

// learn-client.tsx — client component
if (queue.length === 0) {
  return <AllCaughtUp />;
}
```

The `<AllCaughtUp>` component (`frontend/src/components/learn/all-caught-up.tsx`) renders a purpose-built empty-state UI: a message confirming today's session is complete and a link back to the cardgroup list.

### Why empty must not become an error

Treating an empty result as an error has three concrete failure modes:

1. **Unnecessary error boundary activation.** React error boundaries activate on thrown values. Throwing when the array is empty degrades the whole route segment to the error UI, which is designed for infrastructure failures — not for "you're up to date."

2. **Retry loops.** Many client-side error handlers retry on failure. Retrying an empty-list response is pointless: the server will return the same empty list until a card's `due` timestamp passes. A retry loop burns bandwidth and the user sees a spinner where they should see a congratulatory message.

3. **Loss of diagnostic signal.** When empty and real failures both throw, observability tooling counts them together. An on-call engineer investigating error spikes cannot distinguish "everyone finished their cards" from "the DB is down."

### Pattern

Handle the empty-list branch in the component rendering layer, not in the data-fetching layer:

```tsx
// learn-client.tsx
if (queue.length === 0) {
  return <AllCaughtUp />;
}

// Render the normal learning UI when cards are present.
return <section>...</section>;
```

The RSC (`page.tsx`) passes `cardsData.learnNextDueCards` directly to the client component without guarding against zero-length. The empty check lives exactly once in `LearnClient`, which owns the transition from "cards available" to "session complete."

See [`docs/frontend/rsc-error-handling/partial-response-gqlfetch.md`](partial-response-gqlfetch.md) for the `gqlFetch` error contract (what actually constitutes a thrown error) and [`docs/pagination/totalcount-via-separate-count.md`](../../pagination/totalcount-via-separate-count.md) for the related case of an empty Connection in paginated queries.
