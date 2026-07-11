# Shared presentational component, two layouts: add a sibling, not a layout-variant prop

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

## Why

A presentational component is sometimes consumed by two screens that need a
**different layout** but the **same data and actions**. When that happens, the
shape the two screens share is the *data + action contract* — the masked
GraphQL fragment plus the action props — **not the visual markup**. Three ways
to satisfy the second screen exist; only one keeps the contract and the layout
on the right axes:

1. **Mutate the shared component into the new layout** — breaks the original
   consumer's presentation.
2. **Add a `variant`/`layout` prop that swaps the rendered branch** — encodes
   the *visual shape* as runtime state, which is precisely the axis that should
   be a separate component. It balloons an already prop-heavy component, couples
   two unrelated layouts in one file, and makes every future tweak to one layout
   a regression risk for the other.
3. **Add a sibling component that reuses the contract, not the markup** —
   correct. The data + action contract is shared by import; the layout is owned
   per component.

## What

- Leave the shared component **unchanged** for its existing consumer.
- Add a sibling that imports the **same masked fragment** and declares the
  **same action props** (minus any consumer-specific passthroughs the new layout
  does not need — e.g. a list row drops the tile's `className`/`style`
  animation/​width passthroughs). The sibling renders the new layout.
- Preserve every load-bearing `data-testid` the original emitted (e.g. an
  action button's locale-independent test id that e2e targets), so the swap is
  invisible to existing integration and e2e tests.
- The small duplication of the shared action control (e.g. an Import `<Button>`
  block) is the **deliberate, bounded cost** of not coupling the two layouts. Do
  not extract a shared sub-control if doing so forces an edit to the untouched
  shared component. Extract only when a *third* consumer appears — see
  [`subcomponent-extraction-line-count-threshold.md`](subcomponent-extraction-line-count-threshold.md)
  and the type-co-location trigger in
  [`type-placement-in-component-trees.md`](type-placement-in-component-trees.md).

## How to apply

1. Confirm the shared component has a second consumer that must keep its current
   layout (`grep -rn '<SharedComponent' frontend/src` for call sites). If it has
   only one consumer, just change it in place — this rule does not apply.
2. Create the sibling under the same feature directory. Import the shared masked
   fragment and mirror the prop type (drop passthroughs the new layout omits).
3. Swap the call site that needs the new layout to render the sibling; leave the
   other call site on the original.
4. Do **not** add a `variant`/`layout`/`mode` prop to the shared component to
   serve both layouts from one file.
5. Update the shared fragment's docblock to name **both** consumers — after the
   split, the fragment has two unmaskers, and a docblock that names only the
   original rots (the next reader greps for it on the new route and finds
   nothing).

## Worked example

`CatalogListItem` (`frontend/src/app/catalog/catalog-list-item.tsx`) is a sibling
of `CatalogDeckTile` (`frontend/src/app/catalog/catalog-card.tsx`). Both unmask the
`CatalogDeckFields` fragment (`frontend/src/app/catalog/queries.ts`) and accept
`{ node, importing, imported, onImport, labels?, testIdPrefix? }`.
`CatalogListItem` renders the `/catalog` list row (mirroring `AdminMasterRow`);
`CatalogDeckTile` keeps its fixed-width tile layout for the `/onboarding/start`
deck chooser. The `/catalog` migration from a card grid to a list changed only
the catalog call site and the new sibling — `CatalogDeckTile` and `/onboarding/start`
were never touched. The Import `<Button>` block is duplicated between the two on
purpose; a reviewer accepted that as the cost of leaving the shared tile intact.
