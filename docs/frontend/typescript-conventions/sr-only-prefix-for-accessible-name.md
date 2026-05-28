# Composing an accessible name with an `sr-only` prefix span

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When a heading or label must show only the object name visually while screen readers announce a full verb-plus-object phrase, insert an `sr-only` prefix span before the visible text. Tailwind's `sr-only` utility clips and absolutely-positions the text rather than hiding it with `display:none`, so the prefix remains in the DOM and contributes to the element's computed accessible name.

## Why `sr-only` instead of `aria-label`

Overriding the accessible name with a hardcoded `aria-label` on the heading element duplicates state — if the cardgroup name changes, both the visible text node and the `aria-label` attribute must be updated in sync. Using an `sr-only` span keeps a single source of truth: the full accessible name is assembled from the DOM children and updates automatically whenever `cardgroupName` changes.

## Pattern

```tsx
<span
  className="block overflow-hidden text-ellipsis whitespace-nowrap"
  title={cardgroupName}
>
  <span className="sr-only">Batch import into </span>
  {cardgroupName}
</span>
```

The outer `span` carries the truncation classes (`overflow-hidden text-ellipsis whitespace-nowrap`) and the tooltip (`title={cardgroupName}`). The `sr-only` span is an inner sibling of the visible text node. Because Tailwind's `sr-only` positions the element off-screen with `clip: rect(0,0,0,0)` rather than removing it from the accessibility tree, it appears in `heading.textContent` and in the dialog's computed accessible name via `aria-labelledby`.

## What does not work

**`display:none` or `visibility:hidden`** would remove the prefix from the accessibility tree — screen readers would announce only the cardgroup name without the verb.

**Truncation on the `sr-only` span**: never apply `text-ellipsis` or `truncate` classes to the `sr-only` span itself. Those classes set `overflow: hidden` on the visible flow box, but because `sr-only` already clips the element with absolute positioning, the combination can produce unpredictable rendering artifacts on some browsers. Keep truncation on the outer wrapper only.

## Testing

Assert the computed name, not `textContent`:

```ts
expect(screen.getByRole("dialog")).toHaveAccessibleName(/batch import into.*<cardgroupName>/i);
```

`textContent` works incidentally (it does include `sr-only` text) but does not verify the `aria-labelledby` wiring between the dialog and its title. See [`test-aria-contract-via-accessible-name.md`](./test-aria-contract-via-accessible-name.md) for the general principle.

Reference: `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` (call site) and `frontend/src/components/ui/form-sheet.test.tsx` (`"exposes both verb and destination in the heading textContent"` and `"toHaveAccessibleName"` assertions in the `describe.each` block).
