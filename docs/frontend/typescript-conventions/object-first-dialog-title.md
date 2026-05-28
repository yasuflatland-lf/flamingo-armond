# Object-first dialog title for destination-selection sheets

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

Sheet and drawer titles should foreground the highest-uncertainty variable for the action being performed. The dominant convention in this codebase is verb-first (`"Add card"`, `"Edit card"`) because the destination is constant (the current cardgroup) and the verb is what the user needs to confirm. When the destination is the uncertain, high-risk variable — for example, a batch-import sheet that could be opened from any cardgroup — the convention inverts: promote the destination object to the visible title and move the verb into the accessible name only.

## Why the inversion

Verb-first assumes the verb is what the user must consciously confirm. When a sheet's verb is fixed and universally understood (`"Import"`) but the destination can vary across many cardgroups, reading `"Import"` in the header gives no signal about where the data will land. The cardgroup name is the piece of information the user needs to verify before proceeding. Placing it in the title header — the first thing a screen reader announces and the most visually prominent element — means the user can confirm the destination without reading further.

## How to apply

Pass a `ReactNode` to `FormSheet.title` whose visible text shows only the destination object, while an `sr-only` prefix span provides the full verb phrase for screen readers:

```tsx
<FormSheet
  title={
    <span
      className="block overflow-hidden text-ellipsis whitespace-nowrap"
      title={cardgroupName}
    >
      <span className="sr-only">Batch import into </span>
      {cardgroupName}
    </span>
  }
  open={batchImportOpen}
  onOpenChange={setBatchImportOpen}
  size="lg"
>
  ...
</FormSheet>
```

The visible text is `cardgroupName` alone. The computed accessible name of the heading and dialog includes the full `"Batch import into <cardgroupName>"` string because Tailwind's `sr-only` uses position-absolute clipping (not `display:none`), so the prefix is in the DOM and contributes to the computed name.

**This is a deliberate, documented deviation from the verb-first convention used by the `"Add card"` and `"Edit card"` sheets.** It applies when: (a) the verb is a single, unambiguous action (import, copy, move), and (b) a single trigger can target multiple possible destination objects. If both conditions do not hold, prefer verb-first.

The comment in `frontend/src/app/cardgroups/[id]/cards/cards-client.tsx` at the batch-import `<FormSheet>` call site records this decision inline so future readers do not normalize it back to verb-first without understanding the rationale.

See also: [`sr-only-prefix-for-accessible-name.md`](./sr-only-prefix-for-accessible-name.md) for the `sr-only` span technique used to compose the title.
