# Variables shape MUST match between SSR seed and client cache reads

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

Apollo's cache key is built from canonical-stringified variables. Hard-coding `first: 20` in `page.tsx` while the client uses a `PAGE_SIZE` constant is coincidence-only; bumping the constant breaks the seed-then-update pipeline silently. Lift shared connection variables to one module (e.g. `cards/queries.ts` exports `CARDS_PAGE_SIZE`) that both SSR and client import.

**Export the full default-variables object, not just the page-size scalar.** A `PAGE_SIZE` constant alone leaves three call sites (RSC seed, client `useQuery`, mutation `update` callback) free to disagree on which other variables make it into the cache key — `{ first }` vs `{ first, search: null }` vs `{ first: 20 }` all canonicalise to different keys, and the absence of `search: null` in one branch silently splits the cache. Export a `<TYPE>_DEFAULT_VARS` object typed as the query's generated `*QueryVariables`, and require every read/write site to use it (or spread from it):

```ts
// frontend/src/app/cardgroups/queries.ts
export const CARDGROUPS_PAGE_SIZE = 20;
export const CARDGROUPS_DEFAULT_VARS: MyCardgroupsConnectionQueryVariables = {
  first: CARDGROUPS_PAGE_SIZE,
  search: null,
};
```

Three call sites consume it: `page.tsx` `gqlFetch(..., { variables: CARDGROUPS_DEFAULT_VARS })`, the client `useQuery({ variables: searchQuery === null ? CARDGROUPS_DEFAULT_VARS : { ...CARDGROUPS_DEFAULT_VARS, search: searchQuery } })`, and the create-mutation `update` callback `cache.readQuery({ ..., variables: CARDGROUPS_DEFAULT_VARS })`. The TypeScript type assertion makes any future variable added to the query schema (`orderBy`, etc.) a compile-time prompt to decide whether the new variable belongs in the default — silent additions that drift one call site away from the others surface as type errors. Reference: `frontend/src/app/cardgroups/queries.ts` (`CARDGROUPS_DEFAULT_VARS`).
