# `aria-label` on a bare `<div>` is unreliably announced — add an explicit `role`

> Applies to: any `frontend/src/**` use of a shadcn primitive that renders a bare `<div>`/`<span>` (e.g. `Badge`) and carries an `aria-label`.

The shadcn `Badge` renders a plain `<div>` (implicit role `generic`) and spreads `...props` onto it, so `aria-label` passes through:

```tsx
<Badge aria-label={`Mastery stage: ${LABEL[stage]}`} ...>{LABEL[stage]}</Badge>
```

**Why the label can vanish.** Per ARIA, `aria-label` is "not supported" on elements whose role is `generic` (the implicit role of `<div>`/`<span>`). Several screen readers (NVDA, older VoiceOver) do not announce `aria-label` on a generic, non-interactive element — there is no accessible role to attach the name to. The visible text content still reads (a user hears "Learned"), but the contextual prefix ("Mastery stage: …") is silently dropped. This is invisible to tests: `getByLabelText("Mastery stage: Learned")` resolves in jsdom/RTL, so a test passes while a real screen reader announces only the bare word.

**The fix.** Give the element a role that supports naming. For a non-interactive status indicator, `role="img"` is the right choice — the element is announced as an image whose accessible name is the `aria-label`:

```tsx
<Badge role="img" aria-label={`Mastery stage: ${LABEL[stage]}`} ...>{LABEL[stage]}</Badge>
```

**Boundary.** This applies to `aria-label` placed on a shadcn primitive that renders a bare div/span. Interactive primitives (e.g. `Button`) already carry a supporting role and do not need it. If the visible text alone is a sufficient accessible name, prefer dropping the `aria-label` over adding a role; add `role="img"` only when the label carries context the visible text lacks.

Worked example: `frontend/src/components/learn/mastery-badge.tsx`.
