# URL-backed sheet state

How `useSheetSearchParam` represents drawer / sheet open-state in the URL search params so the sheet survives browser back/forward, is deep-linkable, and does not lose listing-page scroll position. Applies to every drawer or sheet that opens *over* a listing page on `/admin/users`, `/admin/roles`, `/profile`, and similar surfaces.

## Why URL state, not component-local state

A drawer that lives in `useState` works for a single in-page open/close, but it loses three properties the product needs:

- **Browser back closes the sheet.** Without URL backing, "back" navigates away from the listing entirely — surprising and destructive when the user just wanted to dismiss the drawer.
- **Deep-link to a specific open sheet.** Support, ops, and admins paste links like `/admin/roles?edit=role-7` to one another. Component-local state cannot carry this.
- **Scroll position survives.** The old "navigate to `/admin/users/<id>/edit` and back" flow re-mounted the listing, losing scroll. A sheet on the same route keeps the listing mounted; URL state lets `router.push` / `router.replace` set `{ scroll: false }` so the listing does not jump.

URL state is the only design that gets all three. The cost is one shared hook (`useSheetSearchParam`) and a discipline about how every consumer reads/writes the two reserved keys.

## The hook API

`frontend/src/lib/url/use-sheet-search-param.ts` exports a single hook:

```ts
type SheetState =
  | { mode: "closed" }
  | { mode: "new" }
  | { mode: "edit"; id: string };

type OpenSheetState = Exclude<SheetState, { mode: "closed" }>;

function useSheetSearchParam(): {
  state: SheetState;
  open: (next: OpenSheetState) => void;
  close: (options?: { refresh?: boolean }) => void;
};
```

The URL contract is:

| URL | `state` |
|---|---|
| `?new=true` | `{ mode: "new" }` |
| `?edit=<id>` | `{ mode: "edit", id }` |
| neither | `{ mode: "closed" }` |

`open()` and `close()` clone the current search params, strip both sheet keys, and write the new pair via `router.push` / `router.replace`. `{ scroll: false }` keeps the listing in place. `close({ refresh: true })` additionally calls `router.refresh()` so the parent RSC re-fetches after a mutation.

## Discriminated union + `Exclude` make illegal `open()` calls unrepresentable

`SheetState` is a discriminated union. Consumers branch on `state.mode` and only see `state.id` after narrowing to `"edit"` — there is no `id: string | null` flat field to misread.

`open()` accepts `OpenSheetState = Exclude<SheetState, { mode: "closed" }>`. The compiler refuses `open({ mode: "closed" })` — that is what `close()` is for. Calling `open()` only opens; calling `close()` only closes. This split is intentional: every other shape would need a runtime guard.

## Mutual exclusion of `?new` and `?edit` is enforced by construction

Both `open()` and `close()` route through `cloneWithoutSheetParams`, which strips both keys before setting the new one. Two consumers cannot interleave a `?new=true` write with an `?edit=<id>` write to produce `?new=true&edit=role-1`. The parser also prefers `new` over `edit` if both are somehow present (the hook tests pin this precedence), but this is a fail-safe — the path that produces both at once is not reachable from the hook itself.

The mode → param-name map is centralized so a future third mode (`"duplicate"`, `"preview"`) does not silently leave the new param behind:

```ts
const SHEET_PARAM_BY_MODE = {
  new: "new",
  edit: "edit",
} as const satisfies Record<Exclude<SheetState["mode"], "closed">, string>;
```

`satisfies` keeps the literal types (so `params.set(SHEET_PARAM_BY_MODE.edit, id)` is a string literal, not a widened `string`) while requiring every non-`closed` mode to be present at compile time. Adding `mode: "duplicate"` without extending this map is a `tsc --noEmit` error.

## Empty `?edit=` value is a silent-fallthrough trap — warn and treat as closed

`URLSearchParams.get("edit")` returns `""` for `?edit=` (key present, value empty). A naive `if (editId)` truthy check lets the empty string fall through to `mode: "closed"`. The user clicks Edit, the URL says `?edit=`, the sheet does not open, and nothing surfaces in DevTools.

The hook explicitly catches this:

```ts
if (editId === "") {
  console.warn("[useSheetSearchParam] ignoring empty ?edit= value");
  return { mode: "closed" };
}
if (editId !== null) {
  return { mode: "edit", id: editId };
}
```

`open()` also throws on `open({ mode: "edit", id: "" })` — this is a developer error (the caller passed an empty string from upstream), and a runtime throw is preferable to a URL write that produces `?edit=` and the same silent fallthrough. The throw is opt-in only at the call site — `open({ mode: "edit", id })` with a non-empty `id` works as documented.

## Singleton-resource sentinel for pages with no per-entity id

Some pages (`/profile`, account settings) edit a singleton aggregate — there is no per-entity id to encode. The natural fit is still `{ mode: "edit", id }`, but `id` becomes a fixed sentinel rather than a real id:

```ts
// frontend/src/app/profile/profile-page-client.tsx
const PROFILE_SHEET_SENTINEL_ID = "self";

const open =
  sheet.state.mode === "edit" && sheet.state.id === PROFILE_SHEET_SENTINEL_ID;

<Button onClick={() => sheet.open({ mode: "edit", id: PROFILE_SHEET_SENTINEL_ID })}>
```

The choice of sentinel matters: pick a value that cannot collide with any real id on the same surface. `"self"` is safe for `/profile` because no other admin page routes entities named `self`; `"true"` would be a footgun because a future entity id of `"true"` would be ambiguous, and the value reads like a boolean flag to anyone scanning the code.

A named constant + load-bearing comment makes the sentinel intent explicit. A bare `id === "self"` literal is harder to grep and easier to misread.

## Lazy-query "called" gate prevents a "not found" flash on first render

When the sheet body lazily fetches its data via `useLazyQuery`, the first render after `?edit=<id>` lands sees:

- `state.mode === "edit"` (URL changed)
- `loading === false` (the `useEffect` that fires the query has not run yet)
- `data === undefined`
- `error === undefined`

A naive sheet body branches on `data ? <Form /> : <NotFound />` and shows the "not found" banner for one frame before the effect commits and the query starts. The gate combines the `called` flag with a `variables.id` equality check (for race safety) into the body's `loading` decision. Both `/admin/users` and `/admin/roles` had identical copies of this logic, so it lives in one shared helper — `frontend/src/lib/url/use-sheet-target-loading.ts`:

```ts
type SheetTargetQueryState = {
  called: boolean;
  variables: { id: string | number } | undefined;
  loading: boolean;
};

function useSheetTargetLoading(
  id: string | null,
  query: SheetTargetQueryState,
): { matchesSheet: boolean; loading: boolean };
```

Consumers pass the currently-open sheet id (or `null`) and the lazy query's `called` / `variables` / `loading` fields:

```ts
const [
  loadRole,
  {
    data: editRoleData,
    loading: loadingEditRole,
    error: editRoleQueryError,
    called: loadRoleCalled,
    variables: loadRoleVariables,
  },
] = useLazyQuery(AdminRoleQuery, { fetchPolicy: "no-cache" });

const { loading: editRoleLoading } = useSheetTargetLoading(editId, {
  called: loadRoleCalled,
  variables: loadRoleVariables,
  loading: loadingEditRole,
});

<EditRoleSheetBody loading={editRoleLoading} role={editRole} ... />
```

The helper reports `loading` until the effect fires the query, the query settles, *and* the result's variables match the currently-open sheet (`matchesSheet`). Only after all three does the body branch on `role ? <Form /> : <NotFound />`. It has its own isolated test (`use-sheet-target-loading.test.ts`) covering the closed / first-render / stale-id / matched / in-flight states.

## Race-guard: variable equality drops stale lazy-query results

A user can click Edit on row `u-1`, then immediately click Edit on row `u-2` before the first query settles. The first response carries `u-1`'s payload but lands when the sheet is open for `u-2`. Without a guard, the sheet shows `u-1`'s data — a confusing UI bug and a security smell on data-sensitive surfaces.

The guard compares the result's id to the sheet's currently-open id:

```ts
const sheetUser = editUser?.id === editUserId ? editUser : null;
```

When `editUserId` has moved on, `editUser?.id !== editUserId` and `sheetUser` is `null` — the body falls back to its `loading` branch until the second query settles. This is the same pattern the `loadMatchesSheet` gate above uses; both look at "does the result match what the URL currently asks for?" — once at the loading-decision level, once at the data-binding level.

## Confirm-on-dismiss wiring: child Form → parent Sheet via callbacks

`FormSheet` opens a discard-confirmation dialog when the user dismisses while `dirty === true` and `submitting === false`. The form lives inside the sheet body and owns the dirty/submitting state; the sheet owns the dismiss flow. Bridging the two requires the form to expose its state via callbacks:

```ts
<ProfileForm
  onDirtyChange={setDirty}              // mirror dirty into the sheet
  onSubmittingChange={setSubmitting}    // mirror submitting into the sheet
  onRegisterReset={(reset) => {
    resetProfileFormRef.current = reset; // expose reset for resetBeforeClose
  }}
  onSaved={() => sheet.close({ refresh: true })}
  onCancel={close}
/>
```

Three of these (`onDirtyChange`, `onSubmittingChange`, `onSaved`) are state-mirror callbacks. `onRegisterReset` is a one-shot handshake that captures the form's reset function into a ref the parent can call before closing. Naming-wise, `on*` for the mirrors stays consistent with the React convention; `onRegisterReset` is a partial outlier but preserves the pattern's visual coherence at the call site.

The callbacks are declared `optional` on `ProfileForm` because the same component is reused outside the sheet (standalone page mode). Sheet consumers must pass all four — type-system enforcement would require a `mode: "standalone" | "embedded"` discriminated union, which was considered but deferred to a future PR; see the type-design review notes for that change.

## URL writes use `push` for open and `replace` for close

`open()` uses `router.push` so browser back closes the sheet (creates a history entry). `close()` uses `router.replace` so dismissing the sheet does not stack a redundant entry. The pair makes the back button intuitive: each open is a navigation, each close just rewrites the current URL.

Both writes set `{ scroll: false }` so the listing's scroll position is preserved across open/close.

## Test patterns

The hook has a small isolated test file (`use-sheet-search-param.test.ts`) covering the discriminated-union states, the mutual-exclusion writes, and the empty-value warn guard. Each sheet consumer also tests:

- `?edit=<id>` deep-link → sheet opens with the data hydrated
- `loading` / `queryError` / `not-found` branches render the right body
- Race-guard: switching `editId` while a query is in flight drops the stale result
- Empty `?edit=` URL warns and stays closed (regression guard for the silent-fallthrough trap)
- Save success calls `close({ refresh: true })` → `replace` + `router.refresh`

Use a `vi.mock("next/navigation", ...)` block that returns mutable `mockPathname` / `mockSearchParamsValue` strings; the tests assert against `mockPush` / `mockReplace` / `mockRefresh` spies. The fixture pattern is identical in `use-sheet-search-param.test.ts`, `admin-roles-client.test.tsx`, `admin-users-client.test.tsx`, and `profile-page-client.test.tsx` — copy from any of those.
