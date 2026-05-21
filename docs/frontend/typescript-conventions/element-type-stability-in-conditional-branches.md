# Element-type stability across conditional branches with the same `key`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When the two branches of a ternary inside a `.map(...)` render with the same `key` but produce different outer element types, React's reconciler treats them as different elements and unmounts the DOM node before mounting the new one. Reusing the same `key` does not save the node — the element type is part of the reconciliation identity. The visible symptoms are CSS transitions that restart, focus state that disappears, and gesture handlers that lose their in-flight state every time the row toggles.

The trap shows up when an "edit" branch is extracted into a child component that renders its own outer `<li>`. The non-editing branch in the parent still renders `<li>` directly, so the two `key`-equal nodes are `<li>` (parent-owned) on one side and `<li>` (component-owned, but reached through `<EditCardRow>`) on the other — different element types from the reconciler's point of view.

```tsx
// Anti-pattern: same key, different element types in the two branches.
// EditCardRow renders its OWN <li>, so React sees:
//   {key: card.id} → <EditCardRow>  (renders a <li>)
//   {key: card.id} → <li>           (rendered directly)
// Different element types → unmount + remount on every edit toggle.
{edges.map((edge) => {
  const card = edge.node;
  return editingId === card.id ? (
    <EditCardRow
      key={card.id}
      card={card}
      submit={(values) => handleUpdate(card.id, values)}
      // ...
    />
  ) : (
    <li key={card.id} className="rounded-md border border-border overflow-hidden">
      <CardRow card={card} /* ... */ />
    </li>
  );
})}

// EditCardRow.tsx (the extracted component)
export function EditCardRow({ card, submit /* ... */ }: EditCardRowProps) {
  return (
    <li className="rounded-md border border-border p-4" onClick={(e) => e.stopPropagation()}>
      <CardForm /* ... */ />
    </li>
  );
}
```

```tsx
// Correct: both branches render <li> directly at the call site; the
// extracted component returns only its content.
{edges.map((edge) => {
  const card = edge.node;
  return editingId === card.id ? (
    <li
      key={card.id}
      className="rounded-md border border-border p-4"
      onClick={(e) => e.stopPropagation()}
    >
      <EditCardRow card={card} submit={(values) => handleUpdate(card.id, values)} /* ... */ />
    </li>
  ) : (
    <li key={card.id} className="rounded-md border border-border overflow-hidden">
      <CardRow card={card} /* ... */ />
    </li>
  );
})}

// EditCardRow.tsx — content only, no <li>.
export function EditCardRow({ card, submit /* ... */ }: EditCardRowProps) {
  return <CardForm /* ... */ />;
}
```

**Why:** React's reconciler keys element identity on the tuple `(type, key)`, not on `key` alone. When the two ternary branches produce `(EditCardRow, "card-id")` and `(li, "card-id")`, the diff between renders is a type change — the engine drops the existing DOM node, runs the unmount path on any descendant that owns subscriptions or effects, and mounts a fresh tree. A `<form>` inside loses its uncontrolled input state, an `:focus` ring disappears, and any CSS transition mid-flight restarts. The cost is invisible in static screenshots but obvious during interactive use.

**How to apply:** when a `.map(...)` ternary lives next to an extracted child component that wraps its content in `<li>` / `<tr>` / `<div role="row">`, the wrap belongs at the call site, not inside the extracted component. The extracted component receives content props and returns the content directly. The call site owns the outer element so both branches share the same element type and React patches in place. Reference: `EditCardRow` in `frontend/src/app/cardgroups/[id]/cards/components/edit-card-row.tsx` (content only) and the row map in `CardsClient` (`frontend/src/app/cardgroups/[id]/cards/cards-client.tsx`) which wraps both branches in `<li key={card.id}>`. The same rule applies to any list-virtualization integration where row components are remounted on type change — keep the outer element type stable across every branch that shares a key.
