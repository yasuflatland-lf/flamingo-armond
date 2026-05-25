# Positive allowlist over negative exclusion in discriminated-union narrowing

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

Given a discriminated union with `kind` tags, two narrowing styles are syntactically valid but semantically opposite:

```tsx
// AVOID: open-ended — every future variant silently passes through.
// Hypothetical example: negative exclusion leaves all future variants unguarded.
const href = action !== null && action.kind !== "cardgroup" ? action.href : "/cards/new";

// PREFER: closed — every future variant must be explicitly added or falls to the default.
const href =
  action !== null && (action.kind === "card-with-group" || action.kind === "card")
    ? action.href
    : "/cards/new";
```

**Why:** extension-by-default is rarely what consumer code intends. A new variant added to the union (e.g. a future `kind: "bulk-card"`) silently slips through the negative-exclusion form because `action.kind !== "cardgroup"` is true for the new variant too. The positive-allowlist form forces the addition to surface as a compile decision: either the new variant belongs in this consumer's allow list (add it) or it does not (the default branch handles it). This is the consumer-side mirror of the type-design analyzer's exhaustiveness pattern.

**How to apply:** any consumer that branches on a discriminated union's `kind` should enumerate the variants it actually wants — never the variants it does not want. The single exception is the type-system-enforced exhaustive `switch` with a `never`-defaulted guard; there, every variant is named and the compiler enforces totality. The canonical example is `handleCreate` in `frontend/src/components/nav/logo-drawer.tsx`, which switches on `createAction.kind` across `"cardgroup"`, `"card-with-group"`, and `"role"` with:

```tsx
default: {
  const _exhaustive: never = createAction;
  console.error("[LogoDrawer] unhandled createAction kind", createAction);
  return;
}
```

Assigning the unmatched value to `_exhaustive: never` turns any future unhandled variant into a compile error, making exhaustiveness a build-time guarantee rather than a runtime convention.
