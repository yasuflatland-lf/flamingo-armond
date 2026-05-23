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
of letting it surface as a blank UI. Reference: `frontend/src/app/admin/users/admin-user-profile-sheet.tsx`
(`handleSave`), `frontend/src/app/admin/users/admin-user-role-row.tsx`
(`handleRoleToggle`), `frontend/src/app/admin/roles/admin-roles-client.tsx`
(`handleCreateSubmit`, `handleEditSubmit`), and
`frontend/src/app/cardgroups/new/new-cardgroup-client.tsx` — all follow the
`const typename = result?.__typename ?? null;` pattern verbatim.

## Alternative: inline narrowing with `payload as unknown as { __typename?: string }` at the fallthrough

The captured-typename pattern can produce a redundant double check when a
later contributor reads `typename === "X"` and adds the corresponding
`payload?.__typename === "X"` to retain access to the narrowed `payload`
fields (`payload.field`, `payload.message`). The intermediate `typename`
binding becomes load-bearing for the fallthrough warn alone and noise
everywhere else. A code-simplifier pass on tier C promotions removed this
shape from `frontend/src/app/profile/profile-form.tsx`,
`frontend/src/app/onboarding/onboarding-form.tsx`,
`frontend/src/app/cards/new/cards-new-client.tsx`,
`frontend/src/app/cardgroups/[id]/cards/cards-client.tsx`,
`frontend/src/app/learn/[cardgroupId]/learn-client.tsx`, and
`frontend/src/components/cardgroups/rename-cardgroup-dialog.tsx` in favour
of inline narrowing at each known variant and an explicit cast at the
fallthrough:

```ts
const payload = result.data?.updateProfile;

if (payload?.__typename === "InputValidationError") {
  setValidationError({ field: payload.field, message: payload.message });
  return;
}
if (payload?.__typename === "UpdateProfileSuccess") {
  router.refresh();
  return;
}
// Unknown variant: null payload or a future union variant the client was not
// regenerated against.
const unknownPayload = payload as unknown as { __typename?: string } | null | undefined;
console.warn("[ProfileForm] unexpected updateProfile payload", {
  typename: unknownPayload?.__typename ?? null,
});
setBannerMessage(ERR_SOMETHING_WRONG);
```

The `as unknown as` cast is load-bearing: TypeScript has narrowed `payload`
to `never` after the exhaustive known cases, so a direct
`payload?.__typename` read is a compile error. The cast widens the type
back to the structural shape needed to read whatever discriminator the
server actually returned. Both patterns satisfy the same diagnostic
invariant — the fallthrough warn carries the real discriminator string,
not `undefined`.

### Choose the inline-cast pattern when

- The component reads variant-specific fields (`payload.message`,
  `payload.field`, `payload.user`) on the happy path. The captured pattern
  forces the redundant `typename === "X" && payload?.__typename === "X"`
  double check; the inline form drops the duplication.
- The known-variant branch already has only one inline-fragment shape
  matching `__typename`, so the additional `const typename` binding adds
  noise for one downstream consumer (the fallthrough warn).

### Choose the captured-typename pattern when

- The known-variant branches do not touch `payload` after `__typename`
  narrowing (e.g. they only call `router.replace(...)` with values from
  closures), so the redundant double check never arises.
- The captured binding is also used for some other purpose at the call
  site (e.g. dispatching a follow-up `useMemo` keyed on the discriminator).

Either pattern satisfies the rule; the trade-off is local. Do not
mechanically migrate one to the other across the codebase — they coexist
and a reviewer flagging the captured form as a regression in a file that
does not have the redundant-double-check shape is misreading the trade-off.
