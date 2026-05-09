# `expect.objectContaining({ message })` is not enough — add a discriminating key

> Part of [`.claude/rules/frontend-typescript-conventions.md`](../frontend-typescript-conventions.md). See the index for related rules.

`Error.prototype.message` is an own (though non-enumerable) property on every `Error` instance. Vitest's `expect.objectContaining` uses `hasOwnProperty` for key checks. Therefore, asserting `expect.objectContaining({ message: expect.any(String) })` against a `console.error` or `console.warn` second argument will pass whether the argument is the intended structured object `{ message, err }` OR a bare `Error` regression. The test is tautologically green and the regression ships silently.

The fix is to include any **own-property key that does not exist on `Error.prototype`** as a discriminating key — `err`, `entry`, `cardgroupId`, `cardId`, `userId`, etc. all qualify. The rule is "any key the structured payload carries that a bare `Error` does not", not specifically `err`.

```ts
// AVOID — passes for both `{ message: "...", err }` AND a bare Error regression.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({ message: expect.any(String) }),
);

// PREFER — the `err` key is absent on a bare Error instance, so a regression fails.
expect(consoleSpy).toHaveBeenCalledWith(
  "[scope] msg",
  expect.objectContaining({
    message: expect.any(String),
    err: expect.anything(),
  }),
);

// ALSO COMPLIANT — any structured-payload-only key works as the discriminator.
// Used in `learn-client.test.tsx`: the `[learn] setLastViewedCardgroup failed`
// payload carries `cardgroupId`, which `Error.prototype` does not.
expect(consoleSpy).toHaveBeenCalledWith(
  "[learn] setLastViewedCardgroup failed",
  expect.objectContaining({ cardgroupId: CG_ID }),
);
```

**Why:** structured log arguments (`{ message, err }`) are deliberately different from a raw `Error`. The assertion must verify the structure matches what was intentionally logged, not just that some string-ish property exists. The rule applies equally to `console.error` and `console.warn` — both are used for structured payloads in this codebase (e.g. `tryGetDuplicateCardInfo` emits a `console.warn` with `{ entry }` rather than a bare string).

**How to apply:** whenever a `console.error` / `console.warn` call passes a structured object as the second argument, the assertion must reference at least one key whose presence on the payload (and absence on `Error.prototype`) the test relies on. Adding `err: expect.anything()` is the most general fix; using a domain-specific key (`cardgroupId`, `entry`, etc.) is equally compliant and often clearer because it documents what the payload actually carries. Reference: `frontend/src/app/cards/new/cards-new-client.test.tsx` `handleCreate` log assertion (uses `err`), `frontend/src/lib/apollo/graphql-errors.test.ts` (uses `entry`), and `frontend/src/app/learn/[cardgroupId]/learn-client.test.tsx` persist-fail assertion (uses `cardgroupId`).
