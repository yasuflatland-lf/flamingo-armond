# JSDoc as the enforcer of "pre-sanitized" invariants when branded types are not used

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

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

### Adjacent rule: raw vs encoded twin fields on the same variant need JSDoc on both

A discriminated-union variant that intentionally exposes both a pre-encoded URL **and** the raw value the URL was built from (e.g. a navigation factory variant carrying `href: string` *and* `cardgroupId: string`) presents two `string`-typed fields the type system cannot tell apart. A future consumer doing `router.push(\`/learn/${action.cardgroupId}\`)` instead of `router.push(action.href)` silently bypasses encoding — the same correctness risk that made the encoded field exist in the first place. Pair each field with JSDoc that names its contract:

```ts
| {
    kind: "card-with-group";
    /** Pre-encoded URL — already URL-safe, route via `router.push(href)` directly. */
    href: string;
    label: "Add new card";
    /**
     * Raw, unencoded cardgroup id (e.g. for display, analytics, or as a React key).
     * Do NOT interpolate into a URL without `encodeURIComponent` — `href` is the
     * correct field for navigation.
     */
    cardgroupId: string;
  }
```

The JSDoc is the only compile-time signal that the two fields have different contracts. Removing either docstring during refactoring is a load-bearing change — treat it the same as removing the "pre-sanitized" JSDoc above. Reference: `frontend/src/components/nav/fab-action.ts` (`FabAction` `card-with-group` variant). The deeper alternative (drop the raw field entirely and force consumers to either re-parse it from `href` or expose a separate decoded helper) is acceptable, but only when no current consumer has a legitimate use for the raw form (e.g. a React `key`, an analytics event payload, a screen-reader label).
