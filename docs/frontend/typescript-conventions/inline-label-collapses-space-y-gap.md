# An inline `<label>` collapses a `space-y-*` wrapper's gap

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

Tailwind's `space-y-*` utility applies a top-margin rule to every **block-flow** sibling inside the container via the `* + *` combinator. A `<label>` element defaults to `display: inline`, so it is not a block-flow sibling — the margin between it and the following field is silently dropped, making the label appear to sit directly on top of the textarea or input with no gap.

## The symptom

A `<div className="space-y-2">` wrapping a `<label>` and a `<Textarea>` shows no visible gap between them, even though `space-y-2` (8 px) is applied. Adding `margin-top` via arbitrary values or extra wrappers treats the symptom rather than the cause.

## The fix

Add `block` to the label's `className`:

```tsx
<label htmlFor="batch-import-payload" className="block text-xs">
  <span className="font-medium text-muted-foreground">Cards to import</span>
  <span className="font-normal text-muted-foreground/70">
    {" — separate each pair with a Tab"}
  </span>
</label>
<Textarea id="batch-import-payload" ... />
```

`block` changes the label's computed display value to `block`, making it a block-flow sibling and restoring the `space-y-*` gap calculation.

## Why this is easy to miss

HTML `<label>` has `display: inline` by default — the same as `<span>` and `<a>`. When a component library or design system renders form labels it often sets `display: block` via a CSS reset or a wrapper class, masking the default. Hand-written labels that skip the component wrapper and apply only text-styling classes (e.g. `text-xs font-medium`) do not get that reset, so the inline default is in effect and `space-y-*` silently does nothing.

Reference: `frontend/src/components/batch-import/batch-import-wizard.tsx`, the `batch-import-payload` label block.
