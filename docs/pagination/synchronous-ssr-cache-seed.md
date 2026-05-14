# Synchronous SSR cache seed in the render body, not in a `useEffect`

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

The SSR-seed-into-Apollo-cache pattern has historically run inside a `useEffect(() => apollo.writeQuery(...), [apollo, initialConnection])` with a `seededRef` boolean to survive Strict Mode's double-mount. Running the seed in a post-render effect leaves a window between first paint and the effect firing during which `useQuery` (cache-first) finds the cache empty and may issue a network round-trip — defeating the point of the SSR seed.

The fix is to write the seed synchronously in the component body, gated by the same `seededRef` boolean. The ref is set synchronously, so Strict Mode's double-invoke still produces exactly one write:

```tsx
// frontend/src/app/cardgroups/cardgroups-client.tsx
const seededRef = useRef(false);

// Seed runs synchronously during render — no useEffect needed.
// CARDGROUPS_DEFAULT_VARS keeps the cache key identical to the SSR seed and
// the client useQuery — any mismatch silently splits the cache.
if (!seededRef.current && initialConnection != null) {
  seededRef.current = true;
  apollo.writeQuery({
    query: MyCardgroupsConnectionDocument,
    variables: CARDGROUPS_DEFAULT_VARS,
    data: { myCardgroupsConnection: initialConnection },
  });
}

const { data, fetchMore, /* ... */ } = useQuery(MyCardgroupsConnectionDocument, {
  variables: queryVariables,
  fetchPolicy: "cache-first",
  /* ... */
});
```

**Why:** "writes during render" is generally an anti-pattern because most writes are observable side effects that schedule re-renders. Apollo's `cache.writeQuery` is the rare exception: the write does not synchronously trigger a re-render in the calling component (`useQuery`'s subscription notifies on the next microtask), and the operation is idempotent — writing the same data to the same cache key twice produces the same cache state. The synchronous `seededRef` guard makes the second invocation a no-op cheap enough to ignore. The post-effect alternative is the worse trade: the effect fires in a microtask after the first paint, so `useQuery`'s first cache-first read happens against an empty cache and may issue an unnecessary network request.

**How to apply:** any RSC-seeded paginated page that lifts initial data into Apollo cache MUST seed synchronously in the render body, gated by a `useRef(false)` boolean. Do not introduce a `useEffect` for this purpose. Confirm the `variables` object in the `writeQuery` is the **same object identity** the client's `useQuery` is keyed on (the shared `<TYPE>_DEFAULT_VARS` const documented in [`variables-shape-must-match.md` § "Variables shape MUST match between SSR seed and client cache reads"](variables-shape-must-match.md)); a mismatched variables shape means `useQuery` reads a different cache entry than the seed wrote. Reference: `frontend/src/app/cardgroups/cardgroups-client.tsx` (`if (!seededRef.current && initialConnection != null)` synchronous seed). This rule pairs with [`cache-modify-skips-nonexistent-fields`](../../.claude/rules/pagination.md#frontend-cache-patterns) (documented in the Frontend cache patterns section): both push cache writes to the seam where the cache-key invariant is enforced.
