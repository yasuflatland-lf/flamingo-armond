# Merging a field's format hint into its `<label>` shifts the hint from accessible description to accessible name

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

An input's accessible **name** is the primary label announced by screen readers when focus arrives. Its accessible **description** is the supplementary hint announced after the name (and only when the AT reads extended details). The two are computed from different DOM attributes: the name from `<label htmlFor>` or `aria-label`; the description from `aria-describedby` pointing at a `<p>` or other element.

When a field's format hint is short and directly useful (e.g. `"— separate each pair with a Tab"`), one editorial option is to inline it inside the `<label>`:

```tsx
<label htmlFor="batch-import-payload" className="block text-xs">
  <span className="font-medium text-muted-foreground">Cards to import</span>
  <span className="font-normal text-muted-foreground/70">
    {" — separate each pair with a Tab"}
  </span>
</label>
```

This moves the hint from the input's `aria-describedby` description into its `aria-labelledby` name. Understand the consequence before applying the pattern.

## The consequence: tests must use `toHaveAccessibleName`, not `toHaveAccessibleDescription`

The prior implementation had `aria-describedby` pointing at a `<p>` element containing the hint. Tests asserting the hint using `toHaveAccessibleDescription(...)` would pass. After the inline merge, the hint text is part of the label, so:

- `toHaveAccessibleDescription("— separate each pair with a Tab")` **fails** — the description is now empty (no `aria-describedby`).
- `toHaveAccessibleName("Cards to import — separate each pair with a Tab")` **passes** — the full label text, including the hint, is the computed name.

When converting existing tests after this merge, replace every `toHaveAccessibleDescription` assertion on the affected field with `toHaveAccessibleName` containing the full label text.

## When to apply the inline-label merge

The merge is appropriate when:

- The hint is a short, always-relevant format rule (a separator, a unit, a required character) rather than a conditional clarification.
- Single-source and spatial proximity are more valuable than the description/name distinction.
- The full name including the hint is still concise enough to be announced comfortably — screen readers announce the full accessible name on focus, so a very long name is disruptive.

For longer, context-sensitive hints (e.g. password-strength rules that vary by policy), keep the hint in a separate element referenced by `aria-describedby` so the description is announced on demand and the name stays brief.

Reference: `frontend/src/components/cardgroups/cardgroup-batch-import-form.tsx` (label for `batch-import-payload`), and the corresponding assertion in `cardgroup-batch-import-form.test.tsx`.
