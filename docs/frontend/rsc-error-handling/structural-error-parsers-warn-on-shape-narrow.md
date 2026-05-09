# Structural error parsers must `console.warn` (not silently `continue`) when the shape narrows wrong

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

A helper like `tryGetDuplicateCardInfo` iterates `err.errors` and skips entries whose extensions do not match the expected discriminator. When the discriminator (`extensions.reason === "CARD_DUPLICATE_FRONT"`) matches but the payload is missing required fields (`existingCardId`, `existingBack`), silently calling `continue` downgrades a partially-valid backend payload to a generic error path with no observable signal. Operators have no way to know a CARD_DUPLICATE_FRONT entry was received but discarded.

Emit a `console.warn` with the raw `extensions` object when the discriminator matches but the shape is wrong:

```ts
if (!ext || ext.code !== "BAD_USER_INPUT" || ext.reason !== "CARD_DUPLICATE_FRONT") continue;
// discriminator matched — now validate required fields
if (typeof existingCardId !== "string" || existingCardId === "") {
  console.warn(
    "[graphql-errors] CARD_DUPLICATE_FRONT entry missing required extension fields",
    { entry: { message: entry.message, extensions: ext } },
  );
  continue;
}
```

**PII review gate:** new extension fields added to any backend error variant appear verbatim in this warn payload. Review every new field against the PII policy before landing — field names like `existingBack` (card content) may carry user-authored text, while `existingCardId` (a UUID) is safe. Reference: `frontend/src/lib/apollo/graphql-errors.ts` (`tryGetDuplicateCardInfo`).
