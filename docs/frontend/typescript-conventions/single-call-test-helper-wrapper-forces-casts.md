# A single-call test-helper wrapper forces type casts — prefer the inline typed-mock idiom

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A test helper that wraps exactly one mock-setup call adds an indirection layer that drops `vi.mocked`'s type inference, and the lost inference resurfaces as a manual cast at every call site. The inline form is both shorter and cast-free.

```ts
// WRONG — the wrapper's parameter/return types are no longer inferred from
// the mocked function, so each call site must re-assert the value's type.
function setAuthStatus(status: string) {
  vi.mocked(headers).mockResolvedValue(
    new Headers({ "x-auth-status": status }) as Awaited<ReturnType<typeof headers>>,
  );
}
setAuthStatus("anonymous"); // cast lives inside the helper, repeated for every overload site

// CORRECT — vi.mocked threads the mocked function's signature through, so the
// Headers value is accepted without any assertion.
vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));
```

**Why the cast appears.** `vi.mocked(headers)` returns a mock typed against `headers`'s real signature, so `mockResolvedValue`/`mockResolvedValueOnce` accept a value assignable to the resolved return type directly. A hand-written wrapper severs that link: the wrapper body holds a bare `mockResolvedValue` whose argument type is no longer narrowed by the surrounding `vi.mocked` chain, so the `Headers` literal must be cast to `Awaited<ReturnType<typeof headers>>` to satisfy the call. The cast is pure indirection cost — it exists only because the wrapper threw away the inference the inline form keeps.

**Scope.** This applies specifically to helpers that wrap a *single* mock call with no added logic. A helper that composes several mock setups, computes fixture data, or encapsulates a multi-step arrange phase still earns its keep — the rule targets the one-line pass-through that trades inference for a name. When the only "value" a wrapper adds is naming a single mock call, inline the call and delete the wrapper.

**Reference.** The cast-free inline idiom is the standard across the converted auth-page tests: `frontend/src/app/admin/users/page.test.tsx`, `frontend/src/app/admin/layout.test.tsx`, `frontend/src/app/login/page.test.tsx`, and `frontend/src/app/cardgroups/page.test.tsx` all call `vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": ... }))` directly. A `setAuthStatus(status)` wrapper introduced in one of these files forced an `as Awaited<ReturnType<typeof headers>>` cast at three sites; removing the wrapper in a simplification pass deleted all three casts, and `tsc --noEmit` confirmed they were never load-bearing.
