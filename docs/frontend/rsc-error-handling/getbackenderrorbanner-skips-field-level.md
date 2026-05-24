# `getBackendErrorBanner` deliberately skips field-level `BAD_USER_INPUT` — use `getBackendFieldErrors` first

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

`getBackendErrorBanner` (`frontend/src/lib/apollo/errors.ts`) returns `undefined` for `BAD_USER_INPUT` errors that carry an `extensions.field`, because those errors are meant to be displayed inline next to the offending field, not in a generic banner. A call site that passes such an error to `getBackendErrorBanner` and displays the result will silently show nothing.

Any UI flow that wants to surface a field-level error inline (instead of a generic banner) MUST explicitly call `getBackendFieldErrors(err)?.<field>` first, then fall back to `getBackendErrorBanner`, then to a generic copy string:

```ts
// correct: check field-level error first, then banner, then generic fallback
const fieldErrors = getBackendFieldErrors(err);
const banner = getBackendErrorBanner(err);
setErrorMessage(
  fieldErrors?.front ??    // field-level inline message
  banner ??                // generic banner (skipped for field-level BAD_USER_INPUT)
  "An unexpected error occurred",
);
```

The three-way fallback ensures every typed error from the backend reaches the UI at the most specific level available, without duplicating the field-level message in a second banner. Reference: `frontend/src/app/cards/new/cards-new-client.tsx` `handleOverwrite`.

## Scope: this applies only to mutations that THROW field errors, not result-union mutations

The rule above is correct only for a mutation whose field validation arrives as a **thrown** `BAD_USER_INPUT` — i.e. a non-union mutation where the usecase returns `*ucerr.ValidationError` and `gqlerr.FromUsecaseError` converts it to a top-level GraphQL error. There, the field detail lands in the `catch` branch, so the `catch` must call `getBackendFieldErrors` first or the inline message is silently lost.

For a mutation whose GraphQL return type is a **result union** — e.g. `AdminEditUserResult = AdminEditUserSuccess | InputValidationError | CannotRevokeOwnAdminRoleError` (`schema/admin.graphql`) — field validation is delivered as **response data**, not a thrown error. The resolver returns a `model.InputValidationError` variant in the data channel (`backend/graph/resolver/admin.resolvers.go`, `AdminEditUser`); the client handles it in the data-handling `switch (payload.__typename)`. Applied naively to such a mutation, a `catch`-branch `getBackendFieldErrors(err)` defense is **dead code**: a field-validation failure never throws, so the `catch` never sees it. The `catch` for a result-union mutation handles only the throw-channel codes — `UNAUTHENTICATED`, `FORBIDDEN`, cancellation, and `INTERNAL`.

```ts
// Result-union mutation: field validation is DATA, handled in the switch.
const payload = result.data?.adminEditUser;
switch (payload?.__typename) {
  case "AdminEditUserSuccess": onSaved(); return;
  case "InputValidationError":            // field error, as data
  case "CannotRevokeOwnAdminRoleError":   // business error, as data
    setSaveError(payload.message); return;
  default: /* unexpected shape → warn + generic copy */
}
// catch handles only throws: auth / cancellation / internal — NOT field validation.
```

Worked example: `frontend/src/app/admin/users/admin-user-profile-sheet.tsx` (`adminEditUser`) routes `InputValidationError` through the `switch` and uses `pickAuthErrorMessage` in the `catch`; it never calls `getBackendFieldErrors` because field errors cannot reach the `catch`.

**The Why:** the data-vs-throw split is a layer-responsibility decision — errors-as-data. Field validation that the schema models as a union variant is application data the resolver returns intentionally, not an exception. Putting a field-error defense in the throw channel for such a mutation is defense in the wrong channel, which is dead code rather than safety. Before adding a `getBackendFieldErrors` call to a mutation's `catch`, confirm the mutation's GraphQL return type is *not* a result union that carries `InputValidationError` as a variant. See [Result Union: "errors as data" pattern](../../backend/error-wrapping/result-union-errors-as-data.md), [Application vs Presentation error responsibility split](../../backend/error-wrapping/application-presentation-error-responsibility.md), and [Outcome-union enforcement](../../backend/error-wrapping/outcome-union-enforcement.md).
