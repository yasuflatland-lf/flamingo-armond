# Positive allowlist over negative exclusion in discriminated-union narrowing

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

Given a discriminated union with `kind` tags, two narrowing styles are syntactically valid but semantically opposite:

```tsx
// AVOID: open-ended — every future variant silently passes through.
const href = action !== null && action.kind !== "cardgroup" ? action.href : "/cards/new";

// PREFER: closed — every future variant must be explicitly added or falls to the default.
// frontend/src/components/nav/header-add-card-link.tsx
const href =
  action !== null && (action.kind === "card-with-group" || action.kind === "card")
    ? action.href
    : "/cards/new";
```

**Why:** extension-by-default is rarely what consumer code intends. A new variant added to the union (e.g. a future `kind: "bulk-card"`) silently slips through the negative-exclusion form because `action.kind !== "cardgroup"` is true for the new variant too. The positive-allowlist form forces the addition to surface as a compile decision: either the new variant belongs in this consumer's allow list (add it) or it does not (the default branch handles it). This is the consumer-side mirror of the type-design analyzer's exhaustiveness pattern.

**How to apply:** any consumer that branches on a discriminated union's `kind` should enumerate the variants it actually wants — never the variants it does not want. The single exception is when the consumer is the type-system-enforced exhaustive switch (e.g. a `never`-defaulted `switch (action.kind)`); there, every variant is named and the compiler enforces totality. Reference: `frontend/src/components/nav/header-add-card-link.tsx` after a review finding that `action.kind !== "cardgroup"` allowed any future variant to be silently treated as a card-form destination.
