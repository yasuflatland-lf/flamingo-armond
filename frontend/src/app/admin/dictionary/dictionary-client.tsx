"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useLazyQuery, useMutation, useQuery } from "@apollo/client/react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import {
  AdminDictionaryCardgroupsQuery,
  UpsertDictionaryMutation,
  ValidateDictionaryQuery,
} from "./queries";

/**
 * Encode a UTF-8 string to base64 using the standard alphabet (with padding).
 * Matches the server contract for `payload` in ValidateDictionaryInput /
 * UpsertDictionaryInput: base64-encoded text, one tab-separated front/back
 * pair per line.
 */
function encodePayload(text: string): string {
  // encodeURIComponent escapes all non-ASCII bytes; unescape maps them back to
  // a byte string so that btoa sees only ASCII characters.
  return btoa(unescape(encodeURIComponent(text)));
}

type ValidationResult = {
  valid: boolean;
  parsedWords: Array<{ front: string; back: string; line: number }>;
  errors: Array<{ line: number; message: string }>;
};

type ImportResult = {
  inserted: number;
  updated: number;
  errors: Array<{ line: number; message: string }>;
};

/**
 * Classify a raw Apollo error into a user-facing banner string.
 * FORBIDDEN gets a specific message because the page renders for any
 * authenticated user (see page.tsx JSDoc for the transitional gate rationale).
 */
function classifyError(err: unknown): string {
  if (!err) return "";
  if (CombinedGraphQLErrors.is(err)) {
    for (const ge of err.errors) {
      if (ge.extensions?.code === "FORBIDDEN") return "Admin role required.";
    }
  }
  return getBackendErrorBanner(err) ?? "An unexpected error occurred. Please try again.";
}

export function DictionaryImportClient() {
  const [cardgroupId, setCardgroupId] = useState<string>("");
  const [payloadText, setPayloadText] = useState<string>("");
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
  // validatedPayload tracks the payloadText value that was in effect when the last
  // successful validate call completed. canImport checks this against the current
  // payloadText to prevent importing a stale/edited payload without re-validating.
  const [validatedPayload, setValidatedPayload] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [bannerError, setBannerError] = useState<string>("");

  const {
    data: cardgroupsData,
    loading: cardgroupsLoading,
    error: cardgroupsError,
  } = useQuery(AdminDictionaryCardgroupsQuery);

  const [runValidate, { loading: validating }] = useLazyQuery(ValidateDictionaryQuery, {
    fetchPolicy: "no-cache",
  });

  const [runUpsert, { loading: upserting }] = useMutation(UpsertDictionaryMutation);

  const cardgroups = cardgroupsData?.myCardgroups ?? [];
  const cardgroupsErrorBanner = getBackendErrorBanner(cardgroupsError);

  async function handleValidate() {
    setBannerError("");
    setValidationResult(null);
    setValidatedPayload(null);
    setImportResult(null);
    const payload = encodePayload(payloadText);
    try {
      const result = await runValidate({ variables: { input: { payload } } });
      if (result.data?.validateDictionary) {
        setValidationResult(result.data.validateDictionary);
        // Record which payloadText was validated so canImport can detect stale edits.
        setValidatedPayload(payloadText);
      }
      if (result.error) {
        setBannerError(classifyError(result.error));
      }
    } catch (err) {
      setBannerError(classifyError(err));
    }
  }

  async function handleImport() {
    if (!cardgroupId) {
      setBannerError("Please select a cardgroup before importing.");
      return;
    }
    setBannerError("");
    setImportResult(null);
    const payload = encodePayload(payloadText);
    try {
      const result = await runUpsert({
        variables: { input: { cardgroupId, payload } },
      });
      if (result.error) {
        setBannerError(classifyError(result.error));
        return;
      }
      if (result.data?.upsertDictionary) {
        setImportResult(result.data.upsertDictionary);
      }
    } catch (err) {
      setBannerError(classifyError(err));
    }
  }

  const canImport =
    validationResult?.valid === true &&
    validationResult.parsedWords.length > 0 &&
    !!cardgroupId &&
    validatedPayload === payloadText;

  return (
    <main className="p-8">
      <div className="mb-6 flex items-center gap-4">
        <Link href="/cardgroups" className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">Dictionary Import</h1>
      </div>

      {cardgroupsErrorBanner && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {cardgroupsErrorBanner}
        </div>
      )}

      {bannerError && (
        <div
          className="mb-4 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
          role="alert"
        >
          {bannerError}
        </div>
      )}

      <div className="space-y-6">
        {/* Cardgroup selector */}
        <div className="space-y-2">
          <label htmlFor="cardgroup-select" className="block text-sm font-medium">
            Target cardgroup
          </label>
          {cardgroupsLoading ? (
            <p className="text-sm text-muted-foreground">Loading cardgroups...</p>
          ) : (
            <select
              id="cardgroup-select"
              value={cardgroupId}
              onChange={(e) => setCardgroupId(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <option value="">Select a cardgroup</option>
              {cardgroups.map((cg) => (
                <option key={cg.id} value={cg.id}>
                  {cg.name}
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Payload textarea */}
        <div className="space-y-2">
          <label htmlFor="payload-textarea" className="block text-sm font-medium">
            Dictionary payload
          </label>
          <p className="text-xs text-muted-foreground">
            One entry per line. Each line: <code>front[tab]back</code>
          </p>
          <textarea
            id="payload-textarea"
            value={payloadText}
            onChange={(e) => setPayloadText(e.target.value)}
            rows={10}
            className="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            placeholder={"apple\tapple (the fruit)\nbanana\ta yellow fruit"}
          />
        </div>

        {/* Action buttons */}
        <div className="flex gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={handleValidate}
            disabled={validating || !payloadText.trim()}
          >
            {validating ? "Validating..." : "Validate"}
          </Button>
          <Button
            type="button"
            variant="brand"
            onClick={handleImport}
            disabled={!canImport || upserting}
          >
            {upserting ? "Importing..." : "Import"}
          </Button>
        </div>

        {/* Validation result */}
        {validationResult && (
          <section className="space-y-4">
            <div
              className={`rounded-md p-3 text-sm ${
                validationResult.valid
                  ? "bg-green-50 text-green-800"
                  : "bg-destructive/10 text-destructive"
              }`}
              role="status"
            >
              {validationResult.valid
                ? `Valid — ${validationResult.parsedWords.length} word${validationResult.parsedWords.length === 1 ? "" : "s"} parsed.`
                : `Invalid — ${validationResult.errors.length} error${validationResult.errors.length === 1 ? "" : "s"} found.`}
            </div>

            {validationResult.parsedWords.length > 0 && (
              <div>
                <h2 className="mb-2 text-sm font-medium uppercase tracking-wide text-muted-foreground">
                  Preview ({validationResult.parsedWords.length} entries)
                </h2>
                <div className="overflow-auto rounded-md border border-border">
                  <table className="w-full text-sm">
                    <thead className="bg-muted/50">
                      <tr>
                        <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                          Line
                        </th>
                        <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                          Front
                        </th>
                        <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                          Back
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {validationResult.parsedWords.map((word) => (
                        <tr key={word.line} className="border-t border-border">
                          <td className="px-3 py-2 text-muted-foreground">{word.line}</td>
                          <td className="px-3 py-2">{word.front}</td>
                          <td className="px-3 py-2">{word.back}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {validationResult.errors.length > 0 && (
              <div>
                <h2 className="mb-2 text-sm font-medium uppercase tracking-wide text-muted-foreground">
                  Parse errors
                </h2>
                <ul className="space-y-1">
                  {validationResult.errors.map((err) => (
                    <li
                      key={`${err.line}-${err.message}`}
                      className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
                    >
                      <span className="font-medium">Line {err.line}:</span> {err.message}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </section>
        )}

        {/* Import result */}
        {importResult && (
          <section>
            {importResult.inserted === 0 &&
            importResult.updated === 0 &&
            importResult.errors.length > 0 ? (
              <div className="rounded-md bg-red-50 p-3 text-sm text-red-800" role="alert">
                Import failed: no rows persisted, {importResult.errors.length} parse error
                {importResult.errors.length === 1 ? "" : "s"}.
              </div>
            ) : (
              <div className="rounded-md bg-green-50 p-3 text-sm text-green-800" role="status">
                Import complete — {importResult.inserted} inserted, {importResult.updated} updated
                {importResult.errors.length > 0 &&
                  `, ${importResult.errors.length} error${importResult.errors.length === 1 ? "" : "s"}`}
                .
              </div>
            )}
            {importResult.errors.length > 0 && (
              <ul className="mt-2 space-y-1">
                {importResult.errors.map((err) => (
                  <li
                    key={`${err.line}-${err.message}`}
                    className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
                  >
                    <span className="font-medium">Line {err.line}:</span> {err.message}
                  </li>
                ))}
              </ul>
            )}
          </section>
        )}
      </div>
    </main>
  );
}
