"use client";

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useApolloClient, useLazyQuery, useMutation } from "@apollo/client/react";
import type { JSX } from "react";
import { useState } from "react";
import { ImportCardsMutation, ValidateCardImportQuery } from "@/app/cardgroups/[id]/cards/queries";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";

/**
 * Encode a UTF-8 string to base64 using the standard alphabet (with padding).
 * Matches the server contract for `payload` in ValidateCardImportInput /
 * ImportCardsInput: base64-encoded text, one tab-separated front/back pair per
 * line.
 */
function encodePayload(text: string): string {
  // encodeURIComponent escapes all non-ASCII bytes; unescape maps them back to
  // a byte string so that btoa sees only ASCII characters.
  return btoa(unescape(encodeURIComponent(text)));
}

type ValidationResult = {
  valid: boolean;
  parsedCards: Array<{ front: string; back: string; line: number }>;
  errors: Array<{ line: number; message: string }>;
};

type ImportResult = {
  inserted: number;
  updated: number;
  errors: Array<{ line: number; message: string }>;
};

/**
 * Classify a raw Apollo error into a user-facing banner string. FORBIDDEN gets
 * a specific message because this form is owner-facing — a FORBIDDEN response
 * means the viewer does not own the target cardgroup.
 */
function classifyError(err: unknown): string {
  if (!err) return "";
  if (CombinedGraphQLErrors.is(err)) {
    for (const ge of err.errors) {
      if (ge.extensions?.code === "FORBIDDEN") {
        return "You can only import into a cardgroup you own.";
      }
    }
  }
  return getBackendErrorBanner(err) ?? "An unexpected error occurred. Please try again.";
}

export function CardgroupBatchImportForm(props: {
  cardgroupId: string;
  cardgroupName: string;
  onImported?: () => void;
}): JSX.Element {
  const { cardgroupId, cardgroupName, onImported } = props;

  const [payloadText, setPayloadText] = useState<string>("");
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
  // validatedPayload tracks the payloadText value that was in effect when the
  // last successful validate call completed. canImport checks this against the
  // current payloadText to prevent importing a stale/edited payload without
  // re-validating.
  const [validatedPayload, setValidatedPayload] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [bannerError, setBannerError] = useState<string>("");

  const apollo = useApolloClient();

  const [runValidate, { loading: validating }] = useLazyQuery(ValidateCardImportQuery, {
    fetchPolicy: "no-cache",
  });

  const [runImport, { loading: importing }] = useMutation(ImportCardsMutation);

  async function handleValidate() {
    setBannerError("");
    setValidationResult(null);
    setValidatedPayload(null);
    setImportResult(null);
    const payload = encodePayload(payloadText);
    try {
      const result = await runValidate({ variables: { input: { payload } } });
      if (result.data?.validateCardImport) {
        setValidationResult(result.data.validateCardImport);
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
    setBannerError("");
    setImportResult(null);
    const payload = encodePayload(payloadText);
    try {
      const result = await runImport({
        variables: { input: { cardgroupId, payload } },
      });
      if (result.error) {
        setBannerError(classifyError(result.error));
        return;
      }
      const data = result.data?.importCards;
      if (!data) {
        setBannerError("An unexpected error occurred. Please try again.");
        return;
      }
      setImportResult(data);
      // Refresh the cards list so the background list and the "N cards" badge
      // reflect the freshly imported rows.
      try {
        await apollo.refetchQueries({ include: [CardsByCardgroupConnectionDocument] });
      } catch (refetchErr) {
        // A failed refetch must not block closing the sheet on a successful
        // import; the next mount/poll re-reads the list. Log and continue.
        console.warn("[CardgroupBatchImportForm] cards refetch failed", {
          name: refetchErr instanceof Error ? refetchErr.name : "unknown",
          cardgroupId,
        });
      }
      if (data.errors.length === 0) {
        // Full success: close the sheet.
        onImported?.();
      }
      // Partial failure (error rows present): keep the form open so the result
      // banner and error rows stay visible.
    } catch (err) {
      setBannerError(classifyError(err));
    }
  }

  const parsedCards = validationResult?.parsedCards ?? [];
  const canImport =
    validationResult?.valid === true && parsedCards.length > 0 && validatedPayload === payloadText;

  return (
    <div className="space-y-6">
      <div className="text-sm">
        <span className="font-medium">Target:</span>{" "}
        <span className="text-muted-foreground">{cardgroupName}</span>
      </div>

      {bannerError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerError}
        </div>
      )}

      <div className="space-y-2">
        <label htmlFor="batch-import-payload" className="block text-sm font-medium">
          Cards to import
        </label>
        <p className="text-xs text-muted-foreground">
          One card per line: an English word, a tab, then its Japanese translation (e.g. pasted from
          a spreadsheet column pair).
        </p>
        <Textarea
          id="batch-import-payload"
          value={payloadText}
          onChange={(e) => setPayloadText(e.target.value)}
          rows={10}
          className="font-mono"
          placeholder={"apple\tapple (the fruit)\nbanana\ta yellow fruit"}
        />
      </div>

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
          disabled={!canImport || importing}
        >
          {importing ? "Importing..." : "Import"}
        </Button>
      </div>

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
              ? `Valid — ${parsedCards.length} card${parsedCards.length === 1 ? "" : "s"} parsed.`
              : `Invalid — ${validationResult.errors.length} error${
                  validationResult.errors.length === 1 ? "" : "s"
                } found.`}
          </div>

          {parsedCards.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-medium uppercase tracking-wide text-muted-foreground">
                Preview ({parsedCards.length})
              </h3>
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
                    {parsedCards.map((card) => (
                      <tr key={card.line} className="border-t border-border">
                        <td className="px-3 py-2 text-muted-foreground">{card.line}</td>
                        <td className="px-3 py-2">{card.front}</td>
                        <td className="px-3 py-2">{card.back}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {validationResult.errors.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-medium uppercase tracking-wide text-muted-foreground">
                Parse errors
              </h3>
              <ul className="space-y-1" role="alert">
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

      {importResult && (
        <section role="status">
          {importResult.inserted === 0 &&
          importResult.updated === 0 &&
          importResult.errors.length > 0 ? (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              Import failed: no cards persisted, {importResult.errors.length} error
              {importResult.errors.length === 1 ? "" : "s"}.
            </div>
          ) : (
            <div className="rounded-md bg-green-50 p-3 text-sm text-green-800">
              Import complete — {importResult.inserted} inserted, {importResult.updated} updated
              {importResult.errors.length > 0 &&
                `, ${importResult.errors.length} error${
                  importResult.errors.length === 1 ? "" : "s"
                }`}
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
  );
}
