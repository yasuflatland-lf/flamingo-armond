# Subcomponent extraction: line-count threshold and layer separation

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A component file that grows past 400 lines is a signal to extract, but extraction has two distinct axes that must be applied in order: **hook extraction** (Application layer — state, network, IO) and **view-fragment extraction** (Presentation layer — pure JSX). The two are orthogonal; applying only one while the file remains over the threshold leaves the second job undone.

## The threshold and the two-phase rule

1. **Hook extraction first.** Move stateful logic, network calls, and IO side effects into a custom hook (e.g. `useCardsConnection`). This is an Application-layer concern per the SRP: the component should describe _what to show_, not _how to fetch it_.
2. **View-fragment extraction second, if still over 400 lines.** After hook extraction, if the component file is still at or above 400 lines, identify the largest pure-presentational JSX block — typically a per-row renderer in a list — and move it to `<feature>/components/<element-noun>.tsx`. Repeat until the host component is under 400 lines.

A worked example: a god component at 727 lines had hook extraction applied first, reducing it to 546 lines — still over the cap. Extracting three view fragments (`CardRow`, `EditCardRow`, `BulkActionBar`) to `cards/components/*.tsx` brought the host to 388 lines. Both phases were necessary.

## Wrong: stop after hook extraction when the file is still over 400 lines

```
// cards/page.tsx — 546 lines after hook extraction.
// The per-row renderer (CardRow) and bulk action bar are still inline.
// The file is responsible for rendering, row state, and multi-select wiring
// simultaneously — two SRP violations remain.
export function CardsPage() {
  const { cards, bulkSelection, ... } = useCardsConnection(args);

  function CardRow({ card }: { card: Card }) { /* 80 lines of JSX */ }
  function BulkActionBar() { /* 60 lines of JSX */ }

  return (
    <>
      {cards.map(c => <CardRow key={c.id} card={c} />)}
      <BulkActionBar />
    </>
  );
}
```

## Right: extract view fragments to `<feature>/components/`

```
// cards/components/card-row.tsx
export interface CardRowProps { card: Card; onEdit: () => void; }
export function CardRow({ card, onEdit }: CardRowProps) { /* pure JSX */ }

// cards/components/bulk-action-bar.tsx
export interface BulkActionBarProps { selectedCount: number; onDelete: () => void; }
export function BulkActionBar(props: BulkActionBarProps) { /* pure JSX */ }

// cards/page.tsx — 388 lines; host is now coordination-only.
import { CardRow } from "./components/card-row";
import { BulkActionBar } from "./components/bulk-action-bar";

export function CardsPage() {
  const { cards, bulkSelection, ... } = useCardsConnection(args);
  return (
    <>
      {cards.map(c => <CardRow key={c.id} card={c} onEdit={...} />)}
      <BulkActionBar selectedCount={bulkSelection.size} onDelete={...} />
    </>
  );
}
```

## Naming and file layout conventions

- Place extracted view fragments under `<feature>/components/<element-noun>.tsx`, not a shared `components/` at the app root unless the fragment is genuinely cross-feature.
- Each extracted file declares its `Props` interface at the top of the file — no separate `types.ts` unless the type is shared by three or more files in the same feature.
- No barrel file (`index.ts`) for feature-local subcomponents. Direct named imports keep dependency tracking explicit and avoid bundler-opaque re-export graphs.
- The extracted component must be a pure presentational function: no `useEffect`, no network calls, no Apollo hooks. State that belongs to presentation (e.g. hover, focus, local open/closed) is allowed; Application-layer state belongs in the hook.

## Why

Hook extraction and view-fragment extraction address different DDD layers. Confusing the two — or stopping after one when both are needed — leaves the host file with multiple responsibilities. A host above 400 lines after hook extraction is emitting a signal: the Presentation layer itself is tangled. The 400-line threshold is a heuristic, not a hard rule, but it is a reliable proxy for "this file is doing more than one thing."
