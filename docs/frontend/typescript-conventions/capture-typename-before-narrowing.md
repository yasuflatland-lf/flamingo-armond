# Capture `__typename` to a local before narrowing on a discriminated union

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

When branching on a GraphQL union's `__typename`, the final "unknown variant"
fallthrough — reached when the server returns a `__typename` the client was not
regenerated against, a `null` payload, or a partial-response null bubble — wants
to log the actual string so the operator can correlate the warning with a
schema-codegen drift. TypeScript will have narrowed `payload?.__typename` to
`never` by the time the fallthrough branch runs, so reading it inline produces
either a compile error or (with `?.`) a stable `undefined` that erases the
diagnostic signal.

Capture the value to a local string **before** the first narrowing check:

```ts
const payload = result.data?.adminUpdateUser;
// Capture before narrowing so the unknown-variant branch still has access
// (TypeScript narrows to `never` after the known cases).
const saveTypename = payload?.__typename ?? null;
if (payload?.__typename === "InputValidationError") {
  setSaveError(payload.message);
  return;
}
if (payload?.__typename === "AdminUpdateUserSuccess") {
  setSaveBanner("Changes saved.");
  return;
}
// Unknown variant: null payload, partial-response null bubble, or a future
// variant the client was not regenerated against.
console.warn("[admin/users/:id/edit] unexpected save payload", {
  typename: saveTypename,
});
setSaveError(ERR_SOMETHING_WRONG);
```

## Why

The narrowed type at the fallthrough is `never`. Two consequences:

1. Reading `payload.__typename` is a compile error — `never` has no members.
2. Reading `payload?.__typename` short-circuits at the optional chain (since
   `payload` may still be `null | undefined`) and always evaluates to `undefined`
   when the payload is non-null but its `__typename` failed every known case.
   The structured warn then logs `{ typename: undefined }`, which serialises to
   no key at all in JSON consoles — the operator sees `{}` and learns nothing.

Capturing to a `const` before the narrowing chain keeps the variable typed as
`string | null` for the fallthrough, so the log carries the actual server
discriminator (e.g. `"FutureVariantWeShipNextWeek"`) instead of `undefined`.

## How to apply

Every client component that dispatches on a GraphQL union's `__typename`:

1. Hoist `const typename = result?.__typename ?? null;` above the first `if`.
2. Pair the unknown-variant fallthrough with a `console.warn` that logs the
   captured string under a stable key (`typename`).
3. The warn payload follows the redaction rule in
   [`docs/frontend/rsc-error-handling/redact-err-message-from-console-payloads.md`](../rsc-error-handling/redact-err-message-from-console-payloads.md):
   log `typename` and stable domain identifiers only; never include `err.message`.

The unknown-variant branch is load-bearing precisely because GraphQL unions are
additive — the backend may ship a new variant before the frontend regenerates,
and the fallthrough is what surfaces the version skew in operator logs instead
of letting it surface as a blank UI. Reference: `frontend/src/app/admin/users/[id]/edit/AdminUserEditClient.tsx`
(`handleSave`, `handleRoleToggle`), `frontend/src/app/admin/roles/[id]/edit/edit-role-client.tsx`,
and `frontend/src/app/admin/roles/new/new-role-client.tsx` — all three follow
the `const typename = result?.__typename ?? null;` pattern verbatim.
