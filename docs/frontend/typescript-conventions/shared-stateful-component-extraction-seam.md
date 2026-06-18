# Unifying two near-identical stateful components: one shared component + thin wrappers, with a behavior-preserving seam

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## Why

Two feature screens sometimes own a **byte-identical stateful component** that
differs only in which mutation it fires, which variables it sends, and which
query it refetches afterward — the rest (state machine, validation, rendering,
every `data-testid`, the i18n namespace) is the same. The duplication is a
divergence-prone liability: a fix to one copy silently skips the other.

This is the opposite axis from the sibling pattern in
[`shared-presentational-component-sibling-over-layout-variant.md`](shared-presentational-component-sibling-over-layout-variant.md):
that rule keeps **two** components because the data+actions are shared but the
**layout** differs. Here the layout and behavior are identical and only the
**network wiring** differs, so the correct move is the inverse — collapse to
**one** shared component consumed by **thin per-feature wrappers**.

The trap is that a careless extraction either (a) leaks the mutation's typed
generics into the shared component's public API, or (b) changes observable
behavior by relocating a side effect across an `await` boundary. The seam below
avoids both.

## What

- **The wrapper owns the typed `useMutation`.** Each wrapper holds its own
  `useMutation(<FeatureMutation>)` and passes the shared component a
  **mutation-only callback**: `onAction(input) => Promise<Result | null>` —
  `throw` is the transport-error channel, `null` is the missing-data channel.
  Keeping `useMutation` in the wrapper preserves the mutation's variable/result
  type safety; a generic `DocumentNode` prop on the shared component would erase
  it.
- **Pass the loading flag through as a prop, do not re-derive it.** The shared
  component cannot own the mutation, so it cannot derive `loading` itself. The
  wrapper passes `busy={mutationLoading}` (the Apollo `useMutation` loading
  flag, verbatim). The shared component reads the prop; it never re-creates a
  parallel in-flight signal.
- **Parameterize the post-mutation refetch by document, do NOT bundle it into
  the action callback.** The shared component owns the refetch orchestration
  (the `apollo.refetchQueries({ include: [refetchDocument] })` call plus its
  swallow-error `try/catch` + structured `console.warn`), parameterized by a
  `refetchDocument` prop. Folding the refetch into `onAction` would move the
  post-success state write (`setResult(...)`) to **after** the awaited refetch,
  delaying the result view by the refetch duration on a partial-failure path —
  an **observable behavior change**. Keeping the refetch in the shared component
  preserves the exact `onAction → setResult → refetch → onDone?` order.
- **Wrappers keep the original export name, path, and public props.** The
  consumer call sites are then **untouched** — the swap is invisible to them and
  to e2e (which targets the preserved `data-testid`s). Verify with
  `git diff --name-only`: a consumer file in the diff means the wrapper's public
  surface drifted.
- **Do not parameterize what does not vary.** If the two copies share the i18n
  namespace, the `data-testid` set, and the result-normalization shape, leave
  them hardcoded in the shared component. Templating an i18n-namespace or
  testId-prefix prop "for flexibility" is YAGNI churn that the actual diff does
  not justify (see [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md)).

## How to apply

1. `grep -rn '<DuplicatedComponent\|<OtherCopy' frontend/src` to confirm exactly
   two consumers, then diff the two source files to enumerate the *real*
   divergence (usually: mutation doc, variable shape, result accessor, refetch
   doc, one display name, one debug-log label).
2. Create the shared component under a feature-neutral directory
   (`components/<shared-noun>/`). Move every shared sub-part with it.
3. Reduce each original file to a ~40-line wrapper that holds `useMutation` and
   passes `onAction` / `busy` / `refetchDocument` / display props. Keep its
   export name and public props.
4. Run the coverage audit: the shared component's test becomes the canonical
   generic suite, so behavioral cases relocate there — and they are *easy to
   drop silently*. See
   [`docs/frontend/testing-convention-narrow-vs-broad-page-tests.md` § "Consolidating components relocates coverage"](../testing-convention-narrow-vs-broad-page-tests.md#consolidating-components-relocates-coverage-audit-the-it-map).
5. Verify the wrapper actually wires `loading → busy`: an isolated shared-component
   test that passes `busy={true}` proves the component *uses* the prop, not that
   the wrapper *supplies* it. A wrapper-level in-flight test is required — see
   [`.claude/rules/scope-discipline.md` § "Render-gating UI props must be wired at the consumer"](../../../.claude/rules/scope-discipline.md#render-gating-ui-props-must-be-wired-at-the-consumer--isolated-component-tests-do-not-prove-reachability).

## Worked example

`BatchImportWizard` (`frontend/src/components/batch-import/batch-import-wizard.tsx`)
unifies the formerly byte-identical `master-batch-import-form.tsx` and
`cardgroups/cardgroup-batch-import-form.tsx`. Its seam is
`onImport(payload) => Promise<ImportResult | null>` + `importing: boolean`
(the wrapper's `useMutation` loading) + `refetchDocument: DocumentNode` +
`targetId` (debug-log context) + `targetName` (confirm-heading). The wrappers
(`MasterBatchImportForm`, `CardgroupBatchImportForm`) keep their export names and
public props, so `master-cards-client.tsx` and
`app/cardgroups/[id]/cards/cards-client.tsx` were not touched. The refetch stays
in the wizard parameterized by `refetchDocument`; an earlier "bundle refetch into
`onImport`" design was rejected because it reordered `setImportResult` after the
refetch. The i18n namespace (`"BatchImport"`), the `batch-import-*` testIds, and
the `ImportResult` shape were identical across both copies and were deliberately
left un-parameterized. Production diff: +560 / −976 (two ~507-line copies → one
~515-line wizard + two ~40-line wrappers).
