# `ReadonlySet<T>` and `<T extends PropertyKey>` for hook-returned identifier sets

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A hook that returns a `Set<T>` of identifiers has two compile-time guards worth applying together. Without them, callers can either (a) mutate the set directly without triggering a React re-render, or (b) instantiate the hook with an object type where each call site constructs fresh objects per render — `Set` then compares by reference identity, not by logical equality, and "the same id" gets added twice.

The first guard types the returned set as `ReadonlySet<T>`. The second constrains the generic parameter to `PropertyKey` (the standard library alias for `string | number | symbol`). Both are zero-runtime-cost and turn two whole classes of misuse into compile errors.

```ts
// Anti-pattern: mutable Set returned, generic unconstrained.
// (a) Caller can .add()/.delete() directly, bypassing React state updates.
// (b) Caller can instantiate with an object type and hit reference-equality
//     semantics in Set, silently storing duplicate "same id" entries.
export interface UseBulkSelectionResult<TId> {
  selectedIds: Set<TId>;
  toggleSelected: (id: TId) => void;
  clearSelection: () => void;
  isSelected: (id: TId) => boolean;
  count: number;
}

export function useBulkSelection<TId = string>(): UseBulkSelectionResult<TId> {
  const [selectedIds, setSelectedIds] = useState<Set<TId>>(new Set());
  // ...
}

// Misuse (a) — silent: no React re-render, UI desync.
result.selectedIds.add("card-1");

// Misuse (b) — silent: each render's { id: "x" } is a fresh object reference.
const sel = useBulkSelection<{ id: string }>();
sel.toggleSelected({ id: "x" }); // toggle on
sel.toggleSelected({ id: "x" }); // toggle on AGAIN (different reference)
```

```ts
// Correct: ReadonlySet at the contract, PropertyKey constraint on the generic.
export interface UseBulkSelectionResult<TId extends PropertyKey> {
  selectedIds: ReadonlySet<TId>;
  toggleSelected: (id: TId) => void;
  clearSelection: () => void;
  isSelected: (id: TId) => boolean;
  count: number;
}

export function useBulkSelection<TId extends PropertyKey = string>(): UseBulkSelectionResult<TId> {
  const [selectedIds, setSelectedIds] = useState<Set<TId>>(new Set());

  const toggleSelected = useCallback((id: TId) => {
    setSelectedIds((prev) => {
      // Copy-on-write: new Set per transition so React sees a new reference.
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);
  // ...
}

// Now both misuses fail compile:
//   result.selectedIds.add("card-1"); // Error: ReadonlySet has no .add
//   useBulkSelection<{ id: string }>(); // Error: {id} does not satisfy PropertyKey
```

**Why `ReadonlySet`:** React state updates require a new reference to trigger a re-render. A caller who does `result.selectedIds.add(x)` mutates the same underlying object the hook is holding — `setState` is never called, the new entry never appears in the DOM, and the hook's internal `count` (derived from `selectedIds.size`) drifts from what consumers see. The `ReadonlySet<T>` type strips `.add` / `.delete` / `.clear` from the public interface; the hook still uses a mutable `Set<T>` internally via copy-on-write. The fix is a type-level guard, not a runtime one — `Object.freeze` would not help because the hook itself needs to replace the set on each transition.

**Why `extends PropertyKey`:** primitive types (`string`, `number`, `symbol`) use value equality inside `Set`, so `"x" === "x"` and `1 === 1` deduplicate correctly. Object types use reference equality, which is almost never what the caller wanted. A component that destructures `{ id }` from an edge and calls `toggleSelected({ id })` builds a fresh object every render; the `Set` accumulates one entry per render rather than tracking a single logical selection. Constraining the generic to `PropertyKey` keeps the hook usable for the common cases (`string` ids, numeric row indices) and rejects the object-keyed shape at the call site. A consumer that has compound keys can serialize them to a string before calling.

**How to apply:** any hook that returns `Set<T>` or `Map<K, V>` as part of its result type should use the `Readonly` variant in the public contract and constrain key generics to `PropertyKey` (sets) or `PropertyKey` keys (maps). Reference: `UseBulkSelectionResult<TId extends PropertyKey>` in `frontend/src/hooks/use-bulk-selection.ts` returns `selectedIds: ReadonlySet<TId>`; the hook's internal `setSelectedIds` callback uses the standard `new Set(prev)` copy-on-write pattern so React sees a new reference on every transition.
