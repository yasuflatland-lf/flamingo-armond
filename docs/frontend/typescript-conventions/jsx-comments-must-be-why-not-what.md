# JSX comments must explain why, not what

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

The project-wide default is to write no comments unless the WHY is non-obvious. This applies equally to JSX `{/* ... */}` block comments. A comment that names a layout region without adding information about a constraint or edge case is pure noise.

## Layout-region labels are redundant

Comments like `{/* Form column */}`, `{/* Decorative panel */}`, or `{/* Mobile-only brand header */}` paraphrase structure that is already visible from three sources:

1. **`data-testid` attributes** — `data-testid="login-form-panel"` names the region unambiguously for both readers and tests.
2. **Tailwind responsive classes** — `lg:hidden` and `max-lg:hidden` declare visibility at a glance; a prose label adds nothing on top.
3. **JSX nesting itself** — a `<section>` containing a `<form>` already communicates "form column" through its structure.

When the comment would not change a reader's understanding of an edge case or hidden constraint, delete it.

## Before / after

```tsx
// Before — comment repeats information already in the class names and nesting
{/* Mobile-only brand header */}
<div className="flex items-center gap-3 lg:hidden">
  <Logo />
</div>
```

```tsx
// After — class names and nesting speak for themselves
<div className="flex items-center gap-3 lg:hidden">
  <Logo />
</div>
```

## When a comment IS justified

Add a comment when removing it would cause a future reader to make the wrong change. Examples:

- A `z-index` value that must stay above a specific third-party overlay — the overlay is not visible in this file.
- An empty element kept intentionally for CSS grid row-alignment — the emptiness looks like a mistake without context.
- A `tabIndex={-1}` added to satisfy a focus-management invariant from an adjacent component.

In all of these the comment answers WHY, not WHAT.

**How to apply:** before committing any JSX comment, ask: "If I deleted this comment, would any reader be more likely to introduce a bug?" If the answer is no, delete it. The test is identical to the project-wide comment rule documented in `CLAUDE.md`.
