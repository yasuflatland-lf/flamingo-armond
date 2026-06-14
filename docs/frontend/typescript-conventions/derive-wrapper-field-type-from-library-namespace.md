# Derive a wrapper hook's re-exposed field type from the wrapped library's namespace type

When a generic wrapper hook re-exposes a field of the library hook it wraps —
e.g. `useConnectionPagination` re-exposing the `refetch` of its internal
`useQuery` — type the field by **indexing the library's own result type**, not
by hand-rolling the signature or loosening it to `unknown`.

```ts
// frontend/src/lib/pagination/use-connection-pagination.ts
import { useQuery } from "@apollo/client/react";

export interface UseConnectionPaginationResult<
  TData,
  TEdge,
  TPageInfo,
  TVars extends OperationVariables,
> {
  // …
  refetch: useQuery.Result<TData, TVars>["refetch"];
}
```

Apollo v4 exposes `useQuery.Result<TData, TVariables>` as a published namespace
type whose `refetch` is
`(variables?: Partial<TVariables>) => Promise<ApolloClient.QueryResult<MaybeMasked<TData>>>`.
Indexing it keeps the wrapper's field **identical to the source of truth**: an
Apollo upgrade that changes the `refetch` signature updates the wrapper for
free, and every consumer keeps a full call-shape check (`refetch()` /
`refetch(vars)`).

## Why not the alternatives

- **Hand-rolled signature** (`refetch: (vars?: Partial<TVars>) => Promise<unknown>`) duplicates Apollo's contract and drifts silently when the library changes it.
- **`unknown` / `any`** discards the call-shape check at every consumer.
- A `ReturnType<typeof useQuery<…>>["refetch"]` instantiation expression is fragile against overloaded hook signatures (`ReturnType` resolves the *last* overload). Prefer the published namespace type (`useQuery.Result`) when the library exports one.

## The result interface must carry `TData`

Indexing `useQuery.Result<TData, TVars>` requires the wrapper's result interface
to be generic over the wrapped data type. Adding `TData` as the **leading**
generic parameter (`UseConnectionPaginationResult<TData, TEdge, TPageInfo, TVars>`)
fans out to every type-alias call site — `UseCardsConnectionResult` and the hook
test's `HookResult` each gained the leading `TData` argument in the same change.
This is the TS-type-alias instance of the constructor-signature / interface
fan-out rule
([`scope-discipline.md` § "Constructor-signature migration"](../../../.claude/rules/scope-discipline.md#constructor-signature-migration-include-resolver-layer-test-files-in-the-pre-flight-grep)):
grep every `UseConnectionPaginationResult<` reference before changing the generic
arity, or the first build fails on the unmigrated alias.

This is the library-hook sibling of
[Derive `optimisticResponse` types from generated mutation types](derive-optimistic-response-types-from-generated.md):
both index a published type (the library's namespace result vs. the codegen
mutation type) instead of re-declaring the shape by hand.
