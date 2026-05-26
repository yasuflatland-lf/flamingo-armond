"use client";

import { useApolloClient, useLazyQuery, useMutation } from "@apollo/client/react";
import { Check, ChevronDown, ChevronRight } from "lucide-react";
import type { JSX } from "react";
import { useState } from "react";
import { ImportCardsMutation, ValidateCardImportQuery } from "@/app/cardgroups/[id]/cards/queries";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Textarea } from "@/components/ui/textarea";
import { CardsByCardgroupConnectionDocument } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { cn } from "@/lib/utils";

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
 * Classify a raw Apollo error into a user-facing banner string, delegating to
 * the shared backend-error banner helper.
 */
function classifyError(err: unknown): string {
  if (!err) return "";
  return getBackendErrorBanner(err) ?? "An unexpected error occurred. Please try again.";
}

/** Pluralize a count's noun without a leading number. */
function plural(n: number, singular: string, pluralForm = `${singular}s`): string {
  return n === 1 ? singular : pluralForm;
}

type Step1ButtonState = {
  hasText: boolean;
  validating: boolean;
  result: ValidationResult | null;
  /** True when the textarea was edited since the last successful validate. */
  isStale: boolean;
};

type Step1ButtonAction = "validate" | "continue";

type Step1ButtonSpec = {
  label: string;
  action: Step1ButtonAction | null;
  disabled: boolean;
};

/**
 * Pure state machine for the step-1 forward button. A string discriminant for
 * `action` (rather than a closure) keeps this trivially unit-testable; the
 * parent maps the discriminant to the real handler.
 *
 * The `valid + isStale` case falls back to "validate" so that editing the
 * textarea after a successful validate forces a re-validate before continuing.
 */
export function resolveStep1Button(state: Step1ButtonState): Step1ButtonSpec {
  if (!state.hasText) {
    return { label: "Validate", action: null, disabled: true };
  }
  if (state.validating) {
    return { label: "Validating...", action: null, disabled: true };
  }
  if (state.result?.valid === true && !state.isStale) {
    return { label: "Continue →", action: "continue", disabled: false };
  }
  return { label: "Validate", action: "validate", disabled: false };
}

/** Breadcrumb showing the two import steps with a back affordance on step 2. */
function ImportStepper(props: {
  current: 1 | 2;
  done: boolean;
  importing: boolean;
  onBack: () => void;
}): JSX.Element {
  const { current, done, importing, onBack } = props;
  return (
    <nav aria-label="Import steps" className="flex items-center justify-between gap-3 text-sm">
      <ol className="flex items-center gap-2">
        <li>
          {current === 2 && done ? (
            <button
              type="button"
              onClick={onBack}
              disabled={importing}
              className="inline-flex items-center gap-1 rounded-md font-medium text-foreground underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 disabled:no-underline"
            >
              <Check aria-hidden="true" className="size-4 text-green-600" />
              Paste &amp; review
            </button>
          ) : (
            <span
              aria-current={current === 1 ? "step" : undefined}
              className={cn(
                "inline-flex items-center rounded-md px-2 py-0.5 font-medium",
                current === 1 ? "bg-brand-primary/10 text-brand-primary" : "text-muted-foreground",
              )}
            >
              Paste &amp; review
            </span>
          )}
        </li>
        <li aria-hidden="true">
          <ChevronRight className="size-4 text-muted-foreground" />
        </li>
        <li>
          <span
            aria-current={current === 2 ? "step" : undefined}
            className={cn(
              "inline-flex items-center rounded-md px-2 py-0.5 font-medium",
              current === 2 ? "bg-brand-primary/10 text-brand-primary" : "text-muted-foreground",
            )}
          >
            Import
          </span>
        </li>
      </ol>
      <span className="text-xs text-muted-foreground">{current} of 2</span>
    </nav>
  );
}

/**
 * Unified collapsible for the validate result: a preview table when valid, an
 * error list when invalid. Collapsed when valid, open when invalid.
 */
function ValidateResult(props: { result: ValidationResult }): JSX.Element {
  const { result } = props;
  const [open, setOpen] = useState<boolean>(!result.valid);

  const triggerLabel = result.valid
    ? `Show preview (${result.parsedCards.length})`
    : open
      ? "Hide errors"
      : `Show errors (${result.errors.length})`;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-md border border-border">
      <CollapsibleTrigger className="flex w-full items-center justify-between px-3 py-2 text-sm font-medium hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">
        <span>{triggerLabel}</span>
        <ChevronDown
          aria-hidden="true"
          className={cn("size-4 transition-transform", open && "rotate-180")}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        {result.valid ? (
          <div className="overflow-auto border-t border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">Line</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">Front</th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">Back</th>
                </tr>
              </thead>
              <tbody>
                {result.parsedCards.map((card) => (
                  <tr key={card.line} className="border-t border-border">
                    <td className="px-3 py-2 text-muted-foreground">{card.line}</td>
                    <td className="px-3 py-2">{card.front}</td>
                    <td className="px-3 py-2">{card.back}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <ul className="space-y-1 border-t border-border p-3" role="alert">
            {result.errors.map((err) => (
              <li
                key={`${err.line}-${err.message}`}
                className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
              >
                <span className="font-medium">Line {err.line}:</span> {err.message}
              </li>
            ))}
          </ul>
        )}
      </CollapsibleContent>
    </Collapsible>
  );
}

export function CardgroupBatchImportForm(props: {
  cardgroupId: string;
  cardgroupName: string;
  onImported?: () => void;
}): JSX.Element {
  const { cardgroupId, cardgroupName, onImported } = props;

  const [step, setStep] = useState<1 | 2>(1);
  const [payloadText, setPayloadText] = useState<string>("");
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
  // validatedPayload tracks the payloadText value that was in effect when the
  // last successful validate call completed. canImport checks this against the
  // current payloadText to prevent importing a stale/edited payload without
  // re-validating. It is also cleared after each import attempt so a re-import
  // requires re-validation.
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
      const data = result.data?.importCards;
      if (!data) {
        setBannerError("An unexpected error occurred. Please try again.");
        return;
      }
      setImportResult(data);
      // Reset the stale-guard so the user must re-validate before importing
      // again — prevents accidental re-upsert of the same payload on a
      // partial-success result where the sheet stays open.
      setValidatedPayload(null);
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
      // Partial failure (error rows present): keep the sheet open so the result
      // banner and error rows stay visible.
    } catch (err) {
      setBannerError(classifyError(err));
    }
  }

  const parsedCards = validationResult?.parsedCards ?? [];
  const canImport =
    validationResult?.valid === true && parsedCards.length > 0 && validatedPayload === payloadText;

  function goBackToStep1() {
    setBannerError("");
    setStep(1);
  }

  const buttonSpec = resolveStep1Button({
    hasText: payloadText.trim() !== "",
    validating,
    result: validationResult,
    isStale: validatedPayload !== payloadText,
  });

  function onStep1ButtonClick() {
    if (buttonSpec.action === "validate") {
      void handleValidate();
    } else if (buttonSpec.action === "continue" && canImport) {
      setImportResult(null);
      setBannerError("");
      setStep(2);
    }
  }

  return (
    <div className="space-y-6">
      <ImportStepper
        current={step}
        done={canImport || step === 2}
        importing={importing}
        onBack={goBackToStep1}
      />

      {bannerError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive" role="alert">
          {bannerError}
        </div>
      )}

      {step === 1 ? (
        <div className="space-y-4">
          <h2 className="text-base font-semibold">Paste your cards</h2>

          <div className="text-sm">
            <span className="font-medium">Target:</span>{" "}
            <span className="text-muted-foreground">{cardgroupName}</span>
          </div>

          <div className="space-y-2">
            <label htmlFor="batch-import-payload" className="block text-sm font-medium">
              Cards to import
            </label>
            <p className="text-xs text-muted-foreground">
              One card per line: an English word, a tab, then its Japanese translation (e.g. pasted
              from a spreadsheet column pair).
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

          {validationResult && (
            <div
              className={cn(
                "rounded-md p-3 text-sm",
                validationResult.valid
                  ? "bg-green-50 text-green-800"
                  : "bg-destructive/10 text-destructive",
              )}
              role="status"
            >
              {validationResult.valid
                ? `✓ Valid — ${parsedCards.length} ${plural(parsedCards.length, "card")} parsed`
                : `✕ Invalid — ${validationResult.errors.length} ${plural(
                    validationResult.errors.length,
                    "error",
                  )}`}
            </div>
          )}

          {validationResult && <ValidateResult result={validationResult} />}

          <div>
            <Button
              type="button"
              variant={buttonSpec.action === "continue" ? "brand" : "outline"}
              onClick={onStep1ButtonClick}
              disabled={buttonSpec.disabled}
            >
              {buttonSpec.label}
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {importResult ? (
            <section className="space-y-4" role="status">
              {importResult.inserted === 0 &&
              importResult.updated === 0 &&
              importResult.errors.length > 0 ? (
                <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
                  Import failed: no cards persisted, {importResult.errors.length}{" "}
                  {plural(importResult.errors.length, "error")}.
                </div>
              ) : (
                <div className="rounded-md bg-green-50 p-3 text-sm text-green-800">
                  Import complete — {importResult.inserted} inserted, {importResult.updated} updated
                  {importResult.errors.length > 0 &&
                    `, ${importResult.errors.length} ${plural(
                      importResult.errors.length,
                      "error",
                    )}`}
                  .
                </div>
              )}
              {importResult.errors.length > 0 && (
                <ul className="space-y-1">
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
              <div className="flex gap-3">
                <Button type="button" variant="outline" onClick={goBackToStep1}>
                  ← Back to edit
                </Button>
                <Button type="button" variant="brand" onClick={() => onImported?.()}>
                  Done
                </Button>
              </div>
            </section>
          ) : (
            <>
              <h2 className="text-base font-semibold">
                Import {parsedCards.length} {plural(parsedCards.length, "card")} into{" "}
                {cardgroupName}?
              </h2>
              <p className="text-sm text-muted-foreground">
                New cards are inserted; existing fronts are updated.
              </p>
              <div className="flex gap-3">
                <Button
                  type="button"
                  variant="outline"
                  onClick={goBackToStep1}
                  disabled={importing}
                >
                  ← Back to edit
                </Button>
                <Button type="button" variant="brand" onClick={handleImport} disabled={importing}>
                  {importing
                    ? "Importing..."
                    : `Import ${parsedCards.length} ${plural(parsedCards.length, "card")}`}
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
