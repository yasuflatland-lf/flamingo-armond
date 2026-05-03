import { CombinedGraphQLErrors } from "@apollo/client/errors";

/**
 * Returns true when `err` is the synthesized GraphQL error thrown by gqlFetch
 * (`server.ts`) and at least one entry in the parsed errors array carries
 * `extensions.code === "UNAUTHENTICATED"`. Substring matching is intentionally
 * NOT used — see .claude/rules/frontend-rsc-error-handling.md.
 */
export function isUnauthenticatedGraphQLError(err: unknown): boolean {
  if (!(err instanceof Error)) return false;
  const prefix = "GraphQL errors: ";
  if (!err.message.startsWith(prefix)) return false;
  try {
    const parsed = JSON.parse(err.message.slice(prefix.length)) as Array<{
      extensions?: { code?: string };
    }>;
    return Array.isArray(parsed) && parsed.some((e) => e?.extensions?.code === "UNAUTHENTICATED");
  } catch {
    return false;
  }
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
  if (!(err instanceof CombinedGraphQLErrors)) return null;
  for (const ge of err.errors) {
    const ext = ge.extensions as Record<string, unknown> | undefined;
    if (!ext) continue;
    if (ext.code !== "BAD_USER_INPUT") continue;
    if (ext.reason !== "CARD_DUPLICATE_FRONT") continue;
    const existingCardId = ext.existingCardId;
    const existingBack = ext.existingBack;
    if (typeof existingCardId !== "string" || existingCardId === "") continue;
    if (typeof existingBack !== "string" || existingBack === "") continue;
    return { existingCardId, existingBack };
  }
  return null;
}
