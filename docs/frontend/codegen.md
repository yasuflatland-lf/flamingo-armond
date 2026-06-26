# Codegen

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Run codegen with:

```bash
pnpm --filter frontend codegen   # frontend only
make codegen                     # repo root — runs backend (gqlgen) + frontend (graphql-codegen)
```

- **Input**: `schema/*.graphql` — shared single source of truth for both sides.
- **Output**: `frontend/src/generated/` (git-ignored). Generated files include `graphql.ts`, `gql.ts`, `fragment-masking.ts`, and `index.ts`.
- **Document discovery**: the client-preset scans `frontend/src/**/*.{ts,tsx}` for `graphql()` tagged templates and includes only the operations actually used.
- **Lifecycle**: a `prebuild` hook in `frontend/package.json` runs codegen automatically before `pnpm build`. In CI, a dedicated `Codegen (graphql-codegen)` step runs between `Install dependencies` and `Biome check`.
- `src/generated/**` is NOT in `tsconfig.json`'s `exclude` — generated output participates in `tsc --noEmit`. If codegen produces broken types, typecheck fails fast rather than hiding behind the exclude.
- **Fragment masking is ON** (client-preset default; `fragment-masking.ts` is generated and in use). A query that spreads `...XFields` exposes a masked ref, not the fields — consumers unmask with `useFragment(XFieldsFragment, ref)` from `@/generated/fragment-masking` (see `app/admin/users/admin-users-client.tsx`, `app/learn/[cardgroupId]/learn-client.tsx`). Test fixtures wrap plain objects with `makeFragmentData(obj, XFieldsFragment)`. Do **not** set `presetConfig: { fragmentMasking: false }` to "simplify" a new fragment: it inlines fragment fields globally and breaks every existing `useFragment` call site (typecheck goes red on `admin-users-client.tsx` first). To extract a shared node fragment for a component used by multiple queries, follow the `admin-users` `useFragment` pattern rather than disabling masking.

## Changing a `graphql()` selection set

Adding or removing a field in a `graphql()` query's selection set is **not** a
pure source edit — it changes the generated `TypedDocumentNode` type. Two
downstream obligations follow, and both fail silently if skipped:

1. **Re-run codegen before `tsc` / tests.** Until codegen regenerates,
   `src/generated/graphql.ts` carries the *old* result type, so the new field is
   absent from the typed result. The `prebuild` hook only runs codegen on a full
   `pnpm build` — a bare `pnpm --filter frontend typecheck` or `test` run uses
   whatever is on disk. Run `pnpm --filter frontend codegen` (or `make codegen`)
   immediately after editing the query.
2. **Add the new field to every `MockedProvider` mock for that query.** Apollo
   matches a mock by document + variables, then returns the mock's `result`
   verbatim; a mock whose result object omits a now-selected field yields
   `undefined` for it at the consumer. Add the field to each **non-empty** result
   object for the query — an `errors: []` / empty-collection mock needs nothing,
   but a mock with populated objects does.

Worked example (issue #685 frontend companion): adding `kind` to
`ValidateCardImportQuery`'s `errors { line message }` selection required a codegen
re-run (so `ValidateCardImportQuery.errors[]` gained `kind: CardImportErrorKind`)
plus adding `kind` to the two non-empty validate mocks in
`batch-import-wizard.test.tsx`; the empty-`errors` mocks were untouched. The
gap this guards against: under-selecting a field that a consumer reads makes the
read return `undefined` at runtime even though the schema field is non-null —
see [`.claude/rules/scope-discipline.md` § "Query under-selection leaves a field `undefined` at the consumer"](../../.claude/rules/scope-discipline.md#query-under-selection-leaves-a-field-undefined-at-the-consumer).

Usage pattern:

```ts
import { graphql } from "@/generated";
const HealthQuery = graphql(`query Health { health }`);
```

The same generated-files policy applies to `backend/graph/generated/` and `backend/graph/model/models_gen.go` on the Go side.

