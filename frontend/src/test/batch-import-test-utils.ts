/**
 * Shared test helpers for batch-import wizard tests.
 *
 * Mirrors the production `encodePayload` in
 * `src/components/batch-import/batch-import-wizard.tsx` so mock payloads
 * match the variables the wizard sends to the validate and import queries.
 */

/**
 * Encode a UTF-8 string to base64, matching the production wizard's encoding.
 */
export function encodePayload(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}
