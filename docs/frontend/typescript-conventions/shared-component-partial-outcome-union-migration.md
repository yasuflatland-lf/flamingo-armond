# Shared form components migrating to outcome unions: additive `validationError` prop, keep legacy `error`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.
> Cross-references [`docs/backend/error-wrapping/outcome-union-enforcement.md`](../../backend/error-wrapping/outcome-union-enforcement.md)
> (the backend-side migration sequence) and
> [`docs/backend/error-wrapping/result-union-errors-as-data.md`](../../backend/error-wrapping/result-union-errors-as-data.md).

## Why

A single form component is often shared between a "create" caller and an
"edit" caller, while outcome-union promotion happens **one mutation at a
time** from the schema-lint allowlist (see
[`outcome-union-enforcement.md`](../../backend/error-wrapping/outcome-union-enforcement.md)).
The two callers diverge during the migration window: the promoted caller
receives a typed `InputValidationError` variant via `__typename` narrowing
and has no need for the Apollo `error` parsing path, while the unpromoted
caller still consumes `BAD_USER_INPUT` via `useMutation`'s `error` and
`getBackendFieldErrors`. The form component cannot pick one path without
breaking the other.

The temptation is to:

1. Migrate the form to consume only the new `validationError` shape (breaks
   the unpromoted caller — its server-side validation messages disappear).
2. Wait until every caller is promoted before changing the form
   (couples N mutation promotions into one PR; busts the 800-line ceiling in
   [`.claude/rules/pr-sizing.md`](../../../.claude/rules/pr-sizing.md)).
3. Branch on `mode` inside the form (couples the form to the caller's
   migration status, which is the wrong axis — the migration is per-mutation,
   not per-mode).

The right shape is **additive**: introduce a new `validationError` prop for
the promoted caller, keep the legacy `error` prop for the unpromoted caller,
and have the form prefer `validationError` for the matching field. The two
props are semantically distinct, so the form's render logic can resolve them
deterministically without consulting any external state.

## What

The shared form accepts both props and resolves them at the field-error
render site:

```tsx
type CardgroupFormProps = {
  mode: "create" | "edit";
  defaultValues: { name: string };
  submit: (values: { name: string }) => Promise<void>;
  submitting?: boolean;
  /** Parent passes Apollo mutation `error` for triage. Used by non-promoted callers (updateCardgroup). */
  error?: unknown;
  /**
   * Typed InputValidationError variant surfaced by outcome-union mutations.
   * When present, takes precedence over `error` for the matching field so
   * the inline field error shows the server message instead of the
   * substring-matched `BAD_USER_INPUT` text.
   */
  validationError?: { field: string; message: string } | null;
};

const fieldErrors = useMemo(() => getBackendFieldErrors(error), [error]);

// In the form.Field render:
<FieldError
  zodErrors={field.state.meta.errors}
  backendError={
    validationError?.field === "name" ? validationError.message : fieldErrors.name
  }
/>
```

The expression `validationError?.field === "name" ? validationError.message : fieldErrors.name`
is the contract: promoted callers pass `validationError` and the form uses
the typed message; unpromoted callers pass `error` and the form falls back
to the parsed `BAD_USER_INPUT` field map. The two props never need to be
considered together — the conditional resolves them at the field's render
site.

The promoted caller (`new-cardgroup-client.tsx`) drives the `validationError`
state from `__typename` narrowing on the mutation result:

```tsx
const [validationError, setValidationError] = useState<{ field: string; message: string } | null>(null);

const result = await createCardgroup({ variables: { input: { name: values.name } } });
const payload = result.data?.createCardgroup;
if (payload?.__typename === "InputValidationError") {
  setValidationError({ field: payload.field, message: payload.message });
  return;
}
// `CreateCardgroupResult` has three variants: `CreateCardgroupSuccess`,
// `InputValidationError`, and `CardgroupLimitReachedError` (carries `limit`
// and `current` when the per-user cardgroup cap is reached, handled as a
// distinct `status: "limit"` branch). Any unrecognized `__typename` falls
// through to a generic error path.

<CardgroupForm mode="create" submit={handleSubmit} validationError={validationError} />
```

The unpromoted caller (e.g. the edit page that consumes `updateCardgroup`,
still on the allowlist) keeps the legacy `error` prop and does not pass
`validationError`:

```tsx
const [updateCardgroup, { loading, error }] = useMutation(UpdateCardgroupMutation);
// ...
<CardgroupForm mode="edit" submit={handleSubmit} submitting={loading} error={error} />
```

## How to apply

When a mutation is promoted to an outcome union but its consuming form is
shared with one or more unpromoted callers:

1. Add `validationError?: { field: string; message: string } | null` to the
   form's prop type — make it optional so existing call sites continue to
   compile without change.
2. At the field-error render site, prefer `validationError.message` when
   `validationError.field === <this-field-name>`; otherwise fall back to
   `getBackendFieldErrors(error)[<this-field-name>]`. Do not branch on the
   form's `mode` prop — the migration is per-mutation, not per-mode.
3. The promoted caller passes `validationError` from `__typename` narrowing
   (see [`capture-typename-before-narrowing.md`](capture-typename-before-narrowing.md))
   and does not pass `error`.
4. The unpromoted caller keeps passing `error` and does not pass
   `validationError`.
5. When the last unpromoted caller is promoted (the allowlist entry for its
   mutation is deleted per the promotion checklist in
   [`outcome-union-enforcement.md`](../../backend/error-wrapping/outcome-union-enforcement.md)),
   remove the legacy `error` prop and its `getBackendFieldErrors` parsing.
   At that point the form has a single error pathway and the migration window
   closes.

## Testing both paths

The shared form's test file must cover three field-error cases so a future
change cannot quietly break either caller:

- `validationError.field === "<form-field>"` → form renders the typed
  server message.
- `validationError.field !== "<form-field>"` AND `error` carries a
  `BAD_USER_INPUT` for the form field → form falls back to the parsed field
  error from `error` (this case proves the legacy path is not bypassed
  when `validationError` is set for a different field).
- `validationError === null` AND `error` carries `BAD_USER_INPUT` for the
  form field → form renders the parsed field error (the legacy
  unpromoted-caller path).

Reference: `frontend/src/components/cardgroups/cardgroup-form.test.tsx`
covers all three cases as a regression suite against the partial-migration
window introduced by promoting `createCardgroup` while `updateCardgroup`
remains on the allowlist.
