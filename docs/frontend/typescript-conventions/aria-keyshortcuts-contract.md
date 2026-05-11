# `aria-keyshortcuts` creates an accessibility contract — omit on disabled elements

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

`aria-keyshortcuts` is a WAI-ARIA attribute that advertises keyboard shortcuts to assistive technology (screen readers). Declaring it on an element creates a contract: the shortcut **must actually work** when the element is reachable, or the attribute becomes actively harmful — a screen reader announces a shortcut that does nothing.

Two rules follow from that contract.

## Values must match `KeyboardEvent.key` exactly

`aria-keyshortcuts` values are compared against `KeyboardEvent.key`, not against display labels or legacy `keyCode` names. Use the exact key identifier strings:

| Wrong | Right |
|---|---|
| `"Arrow Left"` (has space) | `"ArrowLeft"` |
| `"arrow-down"` (kebab) | `"ArrowDown"` |
| `"Right"` (v3-era alias) | `"ArrowRight"` |

A mismatch between the attribute value and the actual `event.key` check in the handler means the screen reader announces a shortcut that does not match the one the handler listens for.

## Omit the attribute on disabled elements — use `isActive ? shortcut : undefined`

When the button is disabled, the shortcut does not work. Leaving `aria-keyshortcuts` on a `disabled` button tells the user a shortcut is available when it is not — violating the contract. Remove the attribute while the button is inactive:

```tsx
<button
  type="button"
  aria-label={`Rate as ${label}`}
  aria-keyshortcuts={disabled ? undefined : shortcut}
  disabled={disabled}
  onClick={() => onRate(direction)}
>
  {/* ... */}
</button>
```

The pattern is the same for any element that can be functionally disabled: pass `undefined` (not an empty string) so the attribute is absent from the DOM rather than present-but-empty.

## Keep the keyboard handler where the shortcut is visually anchored

The `keydown` handler backing `aria-keyshortcuts` belongs in the component that owns the interactive surface. Do not add a second handler for the same keys in a parent component — two independent listeners on `window.addEventListener("keydown", ...)` for the same key produce one event and two callbacks, causing double-swipe or double-action bugs that are invisible during normal use.

The existing guard that makes the single handler safe across queue state changes is documented in [`stabilize-callback-identity-via-useref-mirrors.md`](stabilize-callback-identity-via-useref-mirrors.md): mirror `activeCard` into a ref so the `useCallback` identity stays stable, and early-return when `activeCardRef.current` is `null` — this automatically disables the shortcut when no card is active, without any explicit `disabled` sync.

Reference: `frontend/src/components/learn/swipe-card-stack.tsx` (keyboard handler) and `frontend/src/components/learn/learn-action-bar.tsx` (`aria-keyshortcuts` attributes).
