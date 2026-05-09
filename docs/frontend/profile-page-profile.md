# Profile page (`/profile`)

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

### Page pattern (RSC + client form)

`frontend/src/app/profile/page.tsx` is a React Server Component. It:

1. Calls `createSupabaseServerClient().auth.getUser()` and redirects to `/login` when no session exists.
2. Calls `gqlFetch(MeQuery, { revalidate: 0 })` — `revalidate: 0` opts the response out of the Next cache to avoid serving stale PII.
3. Passes the fetched values as `initial` props to the client component `ProfileForm`.

`frontend/src/app/profile/profile-form.tsx` is a Client Component (`"use client"`). It uses `@tanstack/react-form` with field-level Zod validators (no adapter needed) for client-side validation and Apollo's `useMutation` to call `updateProfile`. On a successful mutation `router.refresh()` is called — this re-evaluates the current RSC subtree and allows client mutations to invalidate server-rendered data without manual cache surgery. Combined with `revalidate: 0` on the server fetch, this gives a simple mutate-then-redisplay flow.

### Authorization forwarding in `gqlFetch`

`frontend/src/lib/apollo/server.ts` reads the Supabase session via `createSupabaseServerClient().auth.getSession()` and forwards `Authorization: Bearer <access_token>` when present. Unauthenticated RSC calls omit the header and receive an `UNAUTHENTICATED` GraphQL error.

`getSession()` reads from the cookie store and does **not** contact the Supabase auth server — it is safe to call per-request. The auth server is only consulted by `getUser()`. Both must destructure and propagate `error`. In `gqlFetch`, an auth-fetch error must **throw** (fail-closed) rather than silently skipping the `Authorization` header — a missing header would produce a silent `UNAUTHENTICATED` response that is indistinguishable from a legitimate anonymous call. The browser-side `authLink` is the only place where a missing session is intentionally fails-open (omits the header without throwing).

### RSC UNAUTHENTICATED redirect pattern

`gqlFetch` throws when the backend returns GraphQL errors. RSC pages wrap the call in `try/catch` and use `redirectIfUnauthenticated(err, target)` from `@/lib/apollo/server-redirect`:

```ts
try {
  data = await gqlFetch(MyQuery, { variables, revalidate: 0 });
} catch (err) {
  redirectIfUnauthenticated(err, "/login"); // never returns
}
```

The helper string-matches `UNAUTHENTICATED` in the error message and calls `redirect(target)`; all other errors are rethrown to the nearest error boundary. Two redirect targets are in use: `/login` for session-expired or no-session cases (checked before `gqlFetch` via `supabase.auth.getUser()`), and `/cardgroups` for cross-user-access on inner pages. See [Backend error-code contract](#backend-error-code-contract) for why these two cases both surface as `UNAUTHENTICATED`.

### Unified admin layout: server-side gate + sidebar

`frontend/src/app/admin/layout.tsx` is the single source of truth for admin access. The RSC layout runs three checks in order before rendering any child route, so a non-admin never sees a flash of admin content:

1. `createSupabaseServerClient().auth.getUser()` — destructure both `data.user` and `error`. A non-null `error` is `throw`n; a null `user` calls `redirect("/")`.
2. `gqlFetch(AdminLayoutMeQuery, { revalidate: 0 })` inside `try/catch` — the catch matches `UNAUTHENTICATED` **and** `FORBIDDEN` substrings on the error message and folds both into `redirect("/")`. Anything else is rethrown to the nearest error boundary.
3. `meData.me?.roles.some((r) => r.name === "admin")` — false ⇒ `redirect("/")`.

Both unauthenticated and non-admin paths redirect to `/` (not `/login`). Sending a logged-in non-admin to `/login` is awkward UX; the home page already routes anonymous visitors through a sign-in CTA.

`redirect()` throws `NEXT_REDIRECT`. Calling it inside a `try/catch` block is fine — Next's error boundary identifies the special throw and acts on it after the catch runs, so `redirect()` may live inside the `catch` arm of step 2 (and does, in the current implementation).

Per-page `getUser()` checks under `admin/dictionary/page.tsx` and `admin/users/page.tsx` are intentionally retained as **defence in depth**. The layout gate is the primary; the per-page check is the belt-and-braces guard against a future refactor that accidentally renders an admin page outside the layout.

The string-match approach (`msg.includes("UNAUTHENTICATED")`) follows the existing `redirectIfUnauthenticated` shape rather than a structured-error type. Replacing it with a typed error envelope is a separate concern; do not introduce a one-off classifier inside the admin layout.

There is no `app/admin/page.tsx` — direct hits on `/admin` (no sub-route) return Next's 404. This is a deliberate accepted tradeoff: every internal entry point links to a specific `/admin/<sub>` route (e.g. the rail's three admin items each link directly to `/admin/users`, `/admin/roles`, `/admin/dictionary`), so a redirect-shim page would have no callers. The two preconditions for keeping `/admin` as a 404: (a) every internal caller links to a specific sub-route (verify with `grep -rn "\"/admin\"\|'/admin'" frontend/src/`); (b) no operator runbook instructs a human to type `/admin` as the entry. If either condition is added later, restore `app/admin/page.tsx` as a server-side `redirect("/admin/users")` shim — the layout gate above runs before the shim, so security posture is unchanged.

### `usePathname()` returns `string | null` despite the typed return

The `next/navigation` `usePathname()` type signature is `string`, but the hook returns `null` during pre-render and outside the App Router runtime. Components that branch on the path (e.g. an active-link sidebar) must guard with `pathname != null && (...)` before reading `.startsWith` or `.substring`, otherwise SSR crashes with a `Cannot read property of null` error that escapes the route's error boundary because layouts are hoisted above the boundary segment.

### Active-link prefix matcher requires the trailing slash

A sidebar that highlights the parent route on nested paths (`/admin/users/123/edit` → "Users" stays active) must compare with `pathname.startsWith(\`${item.href}/\`)`, **not** `pathname.startsWith(item.href)`. Without the trailing slash, `/admin/users-other` matches the `/admin/users` entry as a prefix and double-highlights or wrong-highlights the nav. The full predicate is `pathname === item.href || pathname.startsWith(\`${item.href}/\`)` — the equality arm covers the exact-match case (the trailing slash would otherwise miss it).

### Fragment-less SSR query for pages that share a fragment with their client

`useFragment(...)` from the generated `@apollo/client` runtime is a React Hook and cannot run inside a Server Component. When an RSC seeds the same data the client component reads through a fragment (e.g. `AdminRoleFields`), define a separate inline-fields query for the SSR seed (`AdminRolesPageQuery` in `app/admin/roles/page.tsx`) and pass the result as plain props. The client component still uses the fragment for cache reads and mutation responses; only the SSR boundary needs the un-masked shape.

The seed result is typed as the masked union, so the page must `as unknown as RoleItem[]` before passing it down. The runtime value is already plain — the cast is purely a type-system bridge — so do not invent a runtime un-masking helper just to please the compiler.

### Per-row / per-form field error state must not be shared

`AdminRolesClient` has both an "edit existing row" form and an "add new role" form on the same page. A single `Record<string, string>` for `fieldErrors` cross-contaminates: a `BAD_USER_INPUT` returned by the edit mutation lights up the add row's input as red, and vice versa. Hold one `useState<Record<string, string>>` per form context (`addRowFieldErrors`, `editRowFieldErrors`) and reset both whenever the user starts a new operation. The same rule applies to any future page that mounts multiple field-level forms simultaneously (modal stack, inline-edit table, etc.).

### Surface BAD_USER_INPUT field errors next to the input, not as a banner

`getBackendErrorBanner` deliberately returns `null` for `BAD_USER_INPUT` errors that carry a `field` extension — the banner is reserved for INTERNAL / UNAUTHENTICATED / non-field errors. Pair `getBackendErrorBanner` with `getBackendFieldErrors` and route them to two different render slots:

```tsx
const { banner, fields } = {
  banner: getBackendErrorBanner(err),
  fields: getBackendFieldErrors(err),
};
if (Object.keys(fields).length > 0) setFieldErrors(fields);
if (banner) setError(banner);
if (!banner && Object.keys(fields).length === 0) setError(toMessage(err));  // fallback
```

The fallback to `toMessage(err)` ensures the user is never shown a silent failure. Render the field error directly under the offending input with `aria-invalid` + `aria-describedby` pointing at a `<p role="alert">` so screen readers announce the violation. See `frontend/src/app/admin/roles/AdminRolesClient.tsx` for the reference implementation.

### RSC FORBIDDEN redirect pattern

Admin-only pages (e.g. `/admin/users`, `/admin/users/[id]`) must explicitly handle the `FORBIDDEN` code in their RSC `try/catch`. Unlike `UNAUTHENTICATED` (where `redirectIfUnauthenticated` covers it), an unhandled `FORBIDDEN` rethrows to the nearest error boundary and Next.js renders a 500 — wrong UX for "you are signed in but lack the role". RSC pages call `redirect("/")` (or `/admin` if the user might still belong somewhere) on `FORBIDDEN`:

```ts
try {
  data = await gqlFetch(AdminUsersQuery, { variables, revalidate: 0 });
} catch (err) {
  redirectIfUnauthenticated(err, "/login");
  if (isForbidden(err)) redirect("/");
  throw err;
}
```

Pair the SSR redirect with a **client-side classifier** for mid-session role revocation: a user who lands on the page as admin and then has the role revoked while the page is open will see the next mutation fail with `FORBIDDEN`. A discriminated-union classifier (`classifyQueryError(err): { kind: "forbidden" | "unauthenticated" | "internal" | ... }`) lets the client component branch into a banner or a redirect without inlining string-matches at every call site.

### Mutations that can return `FORBIDDEN` should not use `optimisticResponse`

When a mutation can plausibly return `FORBIDDEN` or `BAD_USER_INPUT` (admin role assignment, self-demotion, etc.), drop `optimisticResponse` entirely. Apollo v3.x rolls back optimistic writes on network errors but not consistently on typed GraphQL errors, so the cache holds the optimistic write while the server has rejected the change. See [`docs/pagination/drop-optimistic-response-typed-errors.md`](../pagination/drop-optimistic-response-typed-errors.md) for the full rule.

### Zod schema convention

Validation schemas live in `frontend/src/schemas/*.ts` and mirror their corresponding GraphQL `Input` types. Example: `frontend/src/schemas/profile.ts` mirrors `UpdateProfileInput`.

The mirror uses `Intl.Segmenter` (UAX #29) for `displayName` and `bio` length checks so that emoji ZWJ sequences count as one character, matching the backend's `rivo/uniseg` with the same UAX #29 standard. Always surface backend `BAD_USER_INPUT` errors (e.g. `extensions.field === "displayName"`) as form-level errors rather than discarding them — see the Form library section for the mapping pattern.

### Form library

We use `@tanstack/react-form` for all forms. No adapter package is needed —
validators are passed directly as per-field Zod schemas.
shadcn's `form.tsx` wrapper was removed — TanStack Form's render-prop
API (`<form.Field>`) does not need it. Forms compose primitive shadcn
components (`Label`, `Input`, `Textarea`) directly.

**`useForm` type inference:** `useForm` has 12 type parameters. Writing `useForm<MyType>(...)` to annotate the form values type does not work — TypeScript cannot infer the remaining 11. Always let the compiler infer from `defaultValues`:

```tsx
// correct — all types inferred from defaultValues
const form = useForm({ defaultValues: { displayName: "", bio: "" }, ... });

// wrong — single explicit type arg leaves 11 params unresolvable → type error
const form = useForm<FormValues>({ ... });
```

**No `validatorAdapter`:** The `useForm` config object in TanStack Form v0.x does not accept a top-level `validatorAdapter` property. Pass Zod schemas directly to each field's `validators` option (see pattern below). The `@tanstack/zod-form-adapter` package is not needed.

#### Pattern

```tsx
// Per-field schemas (declared in @/schemas/profile)
const displayNameSchema = updateProfileSchema.shape.displayName;
const bioSchema = updateProfileSchema.shape.bio;

const form = useForm({
  defaultValues: { displayName: initial.displayName ?? "", bio: initial.bio ?? "" },
  onSubmit: async ({ value }) => { ... },
});

<form.Field
  name="displayName"
  validators={{ onChange: displayNameSchema, onBlur: displayNameSchema }}
>
  {(field) => (
    <>
      <Label htmlFor={field.name}>Display name</Label>
      <Input
        id={field.name}
        value={field.state.value}
        onBlur={field.handleBlur}
        onChange={(e) => field.handleChange(e.target.value)}
      />
      <FieldError zodErrors={field.state.meta.errors} />
    </>
  )}
</form.Field>
```

#### Grapheme cluster validation

`displayName` and `bio` length checks use `Intl.Segmenter` (UAX #29) so that
emoji ZWJ sequences count as one character. The backend uses `rivo/uniseg`
with the same UAX #29 standard, so FE and BE limits agree.

#### Surfacing backend errors in form fields

Backend `BAD_USER_INPUT` errors carry `extensions.field` (see `backend/internal/gqlerr`).
The form maps that field back to the corresponding `<form.Field>` so the user
sees the error inline rather than as a banner. Errors that do not carry a field
(`INTERNAL`, `UNAUTHENTICATED`, network failures, and any other non-field
GraphQL error) are surfaced as a form-level banner with appropriate user-facing
copy. Apollo v4 wraps GraphQL errors in `CombinedGraphQLErrors`; use
`CombinedGraphQLErrors.is(error)` to narrow the type, then read
`error.errors[0]?.extensions?.code` to route between field errors and banner
errors.

**Unhandled rejection from `useMutation`:** Apollo captures the GraphQL error in the `error` state variable automatically, but the promise returned by `mutate(...)` still rejects. Awaiting the promise without a catch causes an unhandled rejection in the browser console. Attach `.catch(console.error)` (or a real error handler) to prevent this while still relying on the `error` state for UI rendering:

```ts
await mutate({ variables }).catch(console.error);
```

Do not swallow the rejection silently with an empty `.catch(() => {})` — that hides unexpected errors (network failures, etc.).

**`void refetch()` swallows `useQuery` refetch rejections.** Unlike `useMutation` where the `error` state captures GraphQL errors, a `refetch()` call from `useQuery` also returns a promise that can reject on network failure. Writing `void refetch()` (or `onClick={() => void refetch()}`) drops that rejection silently — the error never reaches the operator log. Call `.catch` explicitly with a scoped warn:

```ts
refetch().catch((err) => {
  console.warn("[scope] refetch failed", {
    message: err instanceof Error ? err.message : String(err),
    err,
  });
});
```

This is distinct from the `useMutation` unhandled-rejection rule: `useQuery`'s `error` state does update after a failed `refetch`, so the UI error branch still renders — but without the `.catch`, no log record exists for operator triage. Reference: `frontend/src/components/cardgroups/cardgroup-picker-sheet.tsx` Retry button.

**Stale-closure trap with `useMutation` `error` state:** The `error` state from `useMutation` is updated on the next render. Reading it inside the same async handler right after `await mutate(...)` reads the previous closure's stale value. Gate navigation and dialog-close on the `FetchResult` returned by the `await` instead:

```ts
const result = await mutate({ variables }).catch(() => null);
if (result?.data?.updateCardgroup?.cardgroup) {
  router.push(`/cardgroups/${id}`);  // only on confirmed success
}
```

**Dialog open-state tied to mutation result:** Close a destructive-confirm dialog only after verifying `result?.data?.deleteX === true`. Calling `setDialogOpen(false)` synchronously in the confirm `onClick` closes the dialog before the mutation completes, making it impossible to show in-dialog errors.

**Form remount via React `key` to reset fields:** After a successful create-mutation, bump a numeric `key` state variable passed to the form component (`<CardForm key={createFormKey} ...>`). React unmounts and remounts the component, resetting all TanStack Form field state without manual `form.reset()` calls.

**Stay-on-page consecutive add:** A create flow may deliberately *not* navigate after success — the user expects to add several items in a row without leaving the page. Three pieces compose:

1. The submit handler does NOT call `router.push(...)` on success. Instead it (a) clears the form, (b) shows a transient success indicator, and (c) leaves the user with an explicit "Done" link to the natural next page (e.g. `/cardgroups/<id>/cards`). The "Done" link is the only navigation-out affordance, so back-button history stays clean — every add is one history entry on the same URL, not N entries on the destination page.
2. The success indicator is a separate component keyed on a `useState<number | null>` whose value is bumped (`setSuccessKey(Date.now())`) on each successful submit. Because React unmounts-and-remounts on key change, the indicator's `setTimeout(..., 2000)` cleanup runs and a fresh timer starts — two rapid submits get two full 2 s windows, not one extended one. A boolean "isVisible" flag with a single timer would silently extend the previous timer when the second submit lands inside the first's window.
3. Pin "we did not navigate" in a regression-guard test: `expect(mockPush).not.toHaveBeenCalled()` after a successful submit. Without the inverse assertion, a refactor that re-introduces `router.push(...)` passes the happy-path tests (form clears, indicator renders) and silently breaks the consecutive-add UX. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx`.

**`onResetReady?: (resetFn: () => void) => void` — callback inversion to expose `form.reset()` to the parent without giving up `useForm` ownership:** The form's parent (e.g. `CardsNewClient`) needs to call `form.reset()` after the parent's submit-handler succeeds, but `useForm()` must live inside the form component because field validators depend on its identity. `forwardRef + useImperativeHandle` works but pulls the parent into the ref dance for one method. The lighter pattern: the form component accepts an optional `onResetReady` callback and runs `useEffect(() => onResetReady?.(() => form.reset({...})), [form, onResetReady])` once after mount, handing the parent a closure over the live `form`. The parent stores the closure in a `useRef<(() => void) | null>` and calls it from its submit handler. This keeps `useForm` lifecycle inside the child and makes the wiring trivially testable (the parent test does not touch the form's internal state). Reference: `frontend/src/components/cardgroups/card-form.tsx` (`onResetReady`) consumed by `frontend/src/app/cards/new/cards-new-client.tsx`.

**Stable callback identity for child effect deps:** When a child component lists a parent-supplied callback in its `useEffect` dependency array (e.g. `useEffect(() => onResetReady?.(...), [form, onResetReady])`, or a self-dismissing indicator's `useEffect(..., [onTimeout])`), passing an inline arrow (`<Child onX={() => ...} />`) makes the dep change identity on every parent re-render and re-fires the effect. For a `setTimeout`-based dismiss this manifests as the timer never firing — each parent re-render restarts it. Wrap the parent-supplied callback in `useCallback(..., [...])` so the identity is stable across renders that do not change the captured closure. The bug surfaced concretely in `cards-new-client.tsx` where a fire-and-forget `setLastViewedCardgroup` mutation settling caused a parent re-render that reset the success indicator's 2 s timer on every animation frame. This is a React fundamentals issue, not a library quirk — but the failure mode is silent (component does not throw, timers just never elapse).

**Validate-then-mutate flows must invalidate the validation result on input edit.** When a UI splits a server-side check (`validateDictionary`) from a destructive mutation (`upsertDictionary`) and gates the mutation button on the validation result, the validation state goes stale the instant the user edits any input feeding into it. Without explicit invalidation, the user can validate text A, edit to text B, and then submit B against a "valid" gate. The pattern is a `useEffect(() => setValidation(null), [<all input deps>])` whose callback only resets state — the deps array is trigger-only, which Biome flags as `lint/correctness/useExhaustiveDependencies`. Suppress with the inline `biome-ignore` comment as in `frontend/src/app/admin/dictionary/dictionary-client.tsx`.

#### Bio explicit clear UX

`bio` follows tri-state semantics:
- `undefined` — no change
- empty string `""` — explicit clear (sent to mutation, repository writes `bio = ''`)
- non-empty — set

A "Clear bio" button is shown when `bio` is non-empty; clicking it sets the
field to `""` so the next submit clears the column.

### Test stack

`frontend/src/app/profile/profile-form.test.tsx` is the reference for new form tests:

- `@vitest-environment jsdom` directive at the top of the file.
- `MockedProvider` from `@apollo/client/testing/react` stubs Apollo mutations. When testing error-rendering paths that rely on `useMutation`'s `error` field, pass `defaultOptions={{ mutate: { errorPolicy: "all" } }}` to `MockedProvider`; without it `result.errors` in the mock is not surfaced as `error` on the hook.
- `vi.mock("next/navigation", ...)` stubs `useRouter`.
- `@testing-library/react` + `userEvent` drive interaction.
- `expect(element).toBeInTheDocument()` matchers come from `vitest.config.ts` loading `frontend/src/__test-setup__/jest-dom.ts`.

**Real `InMemoryCache` for cache-write/evict tests:** Passing a real `InMemoryCache` to `<MockedProvider cache={cache}>` and pre-seeding it via `cache.writeQuery(...)` lets tests assert the actual cache state after a mutation (`cache.readQuery(...)`) rather than only observable side-effects. Use this to prove that `update` callbacks correctly prepend or evict entries.

**Inverse navigation assertion in failure-path tests:** Use `expect(mockPush).not.toHaveBeenCalled()` in error-path tests to pin down "navigate-on-failure" regressions. Without this assertion, a handler that navigates unconditionally passes happy-path tests but silently breaks on errors.

**`__typename` in `MockedProvider` mocks must match the generated schema type name.** Apollo's normalization layer keys cache entries on `__typename` + identifying fields, and inline-included children are also keyed by their `__typename`. A made-up name (e.g. `"ValidationError"` instead of the schema's `DictionaryValidationError`) is silently degrading: the mock still resolves, but the cache stores a malformed entry and the next lookup misses. Copy the type name from `frontend/src/generated/graphql.ts` rather than guessing.

**`vi.mock` factory props must be typed with `React.ComponentProps<typeof import(...)>`, not `Record<string, unknown>`.** A `vi.mock` factory that types the captured props as `Record<string, unknown>` defeats TypeScript entirely: a future rename of any prop in the real component passes type-check silently because `Record<string, unknown>` accepts any key. Use `React.ComponentProps<typeof import("@/path/to/component").default>` instead — the import path resolves at type-check time, so a renamed prop becomes a compile error in the test. Note: the factory function body itself must also use the same type, not just the capture variable. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx` (`PickerSheetProps` and the `vi.mock` factory for `cardgroup-picker-sheet`).

**E2E locators using `.last()` are positional and fragile.** Playwright's `.last()` silently picks the wrong element when DOM order changes — a new component variant, a different render branch, or a reorder of sibling nodes is enough to flip which element `.last()` resolves to, and the test still passes green while asserting the wrong thing. Instead, scope to the container that is unique to the branch you want:

```ts
const footerScope = page.locator(".mt-6.border-t.pt-4");
const footerLink = footerScope.getByRole("link", { name: /New cardgroup/ });
```

The container CSS class is stable (it comes from the layout, not from dynamic data), so the locator stays correct even when sibling branches add or remove elements. Reference: `frontend/e2e/cardgroups-flow.spec.ts`.

