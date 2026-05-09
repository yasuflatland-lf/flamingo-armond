import { CombinedGraphQLErrors } from "@apollo/client/errors";

/**
 * Returns the structured GraphQL error array thrown by gqlFetch (`server.ts`)
 * when `err.message` carries the "GraphQL errors: ..." prefix; otherwise null.
 * Centralised so every code-extracting helper below shares one parser.
 */
function parseGqlErrors(err: unknown): Array<{ extensions?: { code?: string } }> | null {
  if (!(err instanceof Error)) return null;
  const prefix = "GraphQL errors: ";
  if (!err.message.startsWith(prefix)) return null;
  try {
    const parsed = JSON.parse(err.message.slice(prefix.length));
    return Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

/**
 * Returns true when `err` is the synthesized GraphQL error thrown by gqlFetch
 * (`server.ts`) and at least one entry in the parsed errors array carries
 * `extensions.code === "UNAUTHENTICATED"`. Substring matching is intentionally
 * NOT used — see .claude/rules/frontend-rsc-error-handling.md.
 */
export function isUnauthenticatedGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "UNAUTHENTICATED") ?? false;
}

/**
 * Counterpart to `isUnauthenticatedGraphQLError` for the FORBIDDEN code. Used
 * by RSCs that need to distinguish an authorization failure (route the caller
 * back into the section's listing) from an authentication failure (route to
 * /login).
 */
export function isForbiddenGraphQLError(err: unknown): boolean {
  return parseGqlErrors(err)?.some((e) => e?.extensions?.code === "FORBIDDEN") ?? false;
}

/** Payload returned when the backend detects a duplicate `front` in the same cardgroup. */
export type DuplicateCardInfo = {
  existingCardId: string;
  existingBack: string;
};

/**
 * Inspects a `CombinedGraphQLErrors` from the Apollo client for a
 * `BAD_USER_INPUT / CARD_DUPLICATE_FRONT` entry and returns the existing-card
 * payload when found, or `null` on any other input.
 *
 * Structural parse rationale: substring-matching `err.message` for
 * `"CARD_DUPLICATE_FRONT"` would conflate a genuine duplicate error with any
 * error whose message text happens to include the discriminator (user-supplied
 * content echoed by the backend, telemetry strings, etc.). This function
 * iterates `err.errors`, reads `extensions.code` and `extensions.reason`
 * directly, and returns `null` on any shape mismatch — missing key, wrong
 * type, or empty string — so callers never receive a partially-filled object.
 *
 * See .claude/rules/frontend-rsc-error-handling.md § "Structurally parse
 * GraphQL `extensions.code`".
 */
export function tryGetDuplicateCardInfo(err: unknown): DuplicateCardInfo | null {
  if (!CombinedGraphQLErrors.is(err)) return null;
  for (const entry of err.errors) {
    const ext = entry?.extensions as Record<string, unknown> | undefined;
    if (!ext || ext.code !== "BAD_USER_INPUT" || ext.reason !== "CARD_DUPLICATE_FRONT") continue;
    const { existingCardId, existingBack } = ext;
    if (
      typeof existingCardId !== "string" ||
      existingCardId === "" ||
      typeof existingBack !== "string"
    ) {
      // New extension fields added to CARD_DUPLICATE_FRONT must be reviewed for PII before landing — they appear verbatim in this warn payload.
      console.warn(
        "[graphql-errors] CARD_DUPLICATE_FRONT entry missing required extension fields",
        { entry: { message: entry.message, extensions: ext } },
      );
      continue;
    }
    return { existingCardId, existingBack };
  }
  return null;
}
