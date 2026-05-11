# Structural error parsers must `console.warn` (not silently `continue`) when the shape narrows wrong

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A structural parser iterates `err.errors` and skips entries whose extensions do not match the expected discriminator. When the discriminator matches but the payload is missing required fields, silently calling `continue` downgrades a partially-valid backend payload to a generic error path with no observable signal. Operators have no way to know the entry was received but discarded.

Emit a `console.warn` with the raw `extensions` object when the discriminator matches but the shape is wrong:

```ts
// Example: a helper that extracts a structured payload from a known extension code.
for (const entry of entries) {
  const ext = entry.extensions;
  if (!ext || ext.code !== "BAD_USER_INPUT" || ext.reason !== "EXPECTED_REASON") continue;
  // discriminator matched — now validate required fields
  const entityId = ext.entityId;
  if (typeof entityId !== "string" || entityId === "") {
    console.warn(
      "[graphql-errors] EXPECTED_REASON entry missing required extension fields",
      { entry: { message: entry.message, extensions: ext } },
    );
    continue;
  }
  // ... process valid payload
}
```

The existing helpers in `frontend/src/lib/apollo/graphql-errors.ts` (`isUnauthenticatedGraphQLError`, `isForbiddenGraphQLError`) parse a single well-known extension key (`code`) and return a boolean — they do not carry multi-field payloads, so the warn gate is not applicable there. Apply this rule to any new helper that reads two or more extension fields after a positive discriminator check.

**PII review gate:** new extension fields added to any backend error variant appear verbatim in this warn payload. Review every new field against the PII policy before landing — IDs (UUIDs) are safe; user-authored content fields are not.
