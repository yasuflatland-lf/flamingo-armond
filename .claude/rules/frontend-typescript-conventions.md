# Frontend TypeScript conventions

> Applies to: `frontend/src/**/*.{ts,tsx}`. Cross-cutting type-design rules that affect correctness, security, or testability and are non-obvious from the TypeScript docs alone.

## Required `string | null` over optional `?: string | null` for security-relevant or caller-deliberate props

`prop?: string | null` and `prop: string | null` are not interchangeable. The optional form (`?`) collapses three distinct caller states into two observable outcomes — callers may omit the prop entirely, which is indistinguishable at runtime from an explicit `null` and means the type system does not force the caller to acknowledge the prop's existence. When a prop has security implications (e.g. a redirect destination, a sanitized user-supplied value) or when the calling component must make an explicit choice (pass a value or acknowledge absence), use the required form:

```ts
// AVOID: callers can omit entirely; the prop's existence is unacknowledged.
interface Props { returnTo?: string | null; }

// PREFER: callers must pass something, even if it is null.
interface Props { returnTo: string | null; }
```

The required form surfaces callers that forgot to wire the prop (compile error: "returnTo is missing") rather than silently defaulting to `undefined`. Reference: `frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` (`returnTo: string | null`) after a review finding that the optional form allowed callers to skip the prop and lose the sanitized redirect value without any error.

## JSDoc as the enforcer of "pre-sanitized" invariants when branded types are not used

When a function or component accepts a value that must have crossed a security boundary before being passed (e.g. "this path has been validated as an internal path"), and the project style does not use branded/nominal types, a load-bearing JSDoc comment is the only compile-time signal available. The comment must:

1. State what the caller is responsible for (e.g. "must be a pre-sanitized internal path").
2. State what the receiving side does defensively (e.g. "the receiving page also calls `sanitizeReturnTo`").
3. State what callers must NOT pass (e.g. "do not pass arbitrary user input here").

```ts
/**
 * Path to return to after creating a new cardgroup. Must be a **pre-sanitized
 * internal path** (e.g. `/cards/new`). The receiving page applies
 * `sanitizeReturnTo` defensively, but callers are responsible for not passing
 * arbitrary user input here.
 */
createReturnTo: string;
```

The JSDoc documents a two-layer defence: the caller sanitizes before passing, the receiver sanitizes again on arrival. Both layers are intentional — the "defensive" layer in the receiver is the last-resort guard against a future caller that skips pre-sanitization. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.tsx` (`createReturnTo` prop).

If the project style evolves to allow branded types, replace the JSDoc with a nominal type (e.g. `type InternalPath = string & { readonly __brand: "InternalPath" }`) and a constructor function that calls `sanitizeReturnTo`. Until then, treat the JSDoc as load-bearing — do not remove it during refactoring without adding the branded type.
