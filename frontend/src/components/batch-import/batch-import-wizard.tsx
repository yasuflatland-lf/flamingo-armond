"use client";

import { useApolloClient, useLazyQuery } from "@apollo/client/react";
import type { DocumentNode } from "graphql";
import { ChevronDown } from "lucide-react";
import { useTranslations } from "next-intl";
import type { JSX, ReactNode } from "react";
import { useState } from "react";
import { ValidateCardImportQuery } from "@/app/cardgroups/[id]/cards/queries";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Textarea } from "@/components/ui/textarea";
import type { CardImportErrorKind } from "@/generated/graphql";
import { getBackendErrorBanner } from "@/lib/apollo/errors";
import { cn } from "@/lib/utils";

/**
 * Encode a UTF-8 string to base64 using the standard alphabet (with padding).
 * Matches the server contract for `payload` in `ValidateCardImportInput`,
 * `ImportCardsInput`, and `ImportMasterCardsInput`: base64-encoded text, one
 * tab-separated front/back pair per line.
 */
function encodePayload(text: string): string {
  // encodeURIComponent escapes all non-ASCII bytes; unescape maps them back to
  // a byte string so that btoa sees only ASCII characters.
  return btoa(unescape(encodeURIComponent(text)));
}

type ValidationResult = {
  valid: boolean;
  parsedCards: Array<{ front: string; back: string; line: number }>;
  errors: Array<{ line: number; message: string; kind: CardImportErrorKind }>;
};

export type ImportResult = {
  inserted: number;
  updated: number;
  errors: Array<{ line: number; message: string; kind: CardImportErrorKind }>;
};

/**
 * `DUPLICATE` is non-fatal: the row still persisted via last-write-wins, so it
 * reads as a warning. Every other kind dropped a row and reads as an error.
 */
function isWarningKind(kind: CardImportErrorKind | undefined): boolean {
  return kind === "DUPLICATE";
}

/**
 * Classify a raw Apollo error into a user-facing banner string, delegating to
 * the shared backend-error banner helper.
 */
function classifyError(err: unknown, fallback: string): string {
  if (!err) return "";
  return getBackendErrorBanner(err) ?? fallback;
}

type Step1ButtonState = {
  hasText: boolean;
  validating: boolean;
  result: ValidationResult | null;
  /**
   * True when the textarea was edited since the last successful validate. Only
   * inspected when `result?.valid === true`; it is ignored on every other
   * branch, so `{ result: null, isStale: true }` is a meaningless but harmless
   * input combination.
   */
  isStale: boolean;
};

type Step1ButtonAction = "validate" | "continue";

/**
 * Discriminated on `action`: a button with no action is always disabled, and a
 * button with an action is always enabled. This makes the phantom
 * `{ action: null, disabled: false }` state unrepresentable.
 */
type Step1ButtonSpec =
  | { action: null; labelKey: "validate" | "validating"; disabled: true }
  | { action: Step1ButtonAction; labelKey: "validate" | "import"; disabled: false };

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
    return { labelKey: "validate", action: null, disabled: true };
  }
  if (state.validating) {
    return { labelKey: "validating", action: null, disabled: true };
  }
  if (state.result?.valid === true && !state.isStale) {
    return { labelKey: "import", action: "continue", disabled: false };
  }
  return { labelKey: "validate", action: "validate", disabled: false };
}

/** Quiet 2-segment progress bar showing the two import steps with a back affordance on step 2. */
function ImportStepper(props: {
  current: 1 | 2;
  importing: boolean;
  onBack: () => void;
}): JSX.Element {
  const { current, importing, onBack } = props;
  const t = useTranslations("BatchImport");
  const caption = current === 1 ? t("pasteAndReview") : t("import");
  return (
    <nav aria-label={t("importStepsAriaLabel")} className="flex items-center gap-2">
      {current === 2 ? (
        <button
          type="button"
          onClick={onBack}
          disabled={importing}
          aria-label={t("pasteAndReview")}
          className="-my-2 py-2 disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          <span className="block h-1 w-10 rounded-full bg-brand-primary" />
        </button>
      ) : (
        <div className="h-1 w-10 rounded-full bg-brand-primary" />
      )}
      <div
        className={cn("h-1 w-10 rounded-full", current === 2 ? "bg-brand-primary" : "bg-muted")}
      />
      <span className="ml-1 text-xs text-muted-foreground">{caption}</span>
      <span className="sr-only">{t("stepOf", { current })}</span>
    </nav>
  );
}

/**
 * Renders a list of line-level import/validation messages. A `DUPLICATE`-kind
 * row reads as a non-fatal warning (amber — the row still persisted via
 * last-write-wins); every other kind, and any row without a kind (validation
 * step), reads as a blocking error (red).
 */
function ErrorList(props: {
  errors: Array<{ line: number; message: string; kind?: CardImportErrorKind }>;
  className?: string;
  role?: string;
}): JSX.Element {
  const { errors, className, role } = props;
  const t = useTranslations("BatchImport");
  return (
    <ul className={cn("space-y-1", className)} role={role}>
      {errors.map((err) => (
        <li
          key={`${err.line}-${err.message}`}
          className={cn(
            "rounded-md px-3 py-2 text-sm",
            isWarningKind(err.kind)
              ? "bg-amber-50 text-amber-800"
              : "bg-destructive/10 text-destructive",
          )}
        >
          <span className="font-medium">{t("errorLine", { line: err.line })}</span> {err.message}
        </li>
      ))}
    </ul>
  );
}

/**
 * Unified collapsible for the validate result: a preview table when valid, an
 * error list when invalid. Collapsed when valid, open when invalid.
 */
function ValidateResult(props: { result: ValidationResult }): JSX.Element {
  const { result } = props;
  const t = useTranslations("BatchImport");
  const [open, setOpen] = useState<boolean>(!result.valid);

  let triggerLabel: string;
  if (result.valid) {
    triggerLabel = t("showPreview", { count: result.parsedCards.length });
  } else if (open) {
    triggerLabel = t("hideErrors");
  } else {
    triggerLabel = t("showErrors", { count: result.errors.length });
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-md border border-border">
      <CollapsibleTrigger
        className="flex w-full items-center justify-between px-3 py-2 text-sm font-medium hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        data-testid="batch-import-preview-toggle"
      >
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
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                    {t("previewLine")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                    {t("previewFront")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium text-muted-foreground">
                    {t("previewBack")}
                  </th>
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
          <ErrorList errors={result.errors} className="border-t border-border p-3" role="alert" />
        )}
      </CollapsibleContent>
    </Collapsible>
  );
}

/**
 * Two-slot wizard footer: a secondary/back affordance on the left and the
 * primary/forward action on the right. The primary stays pinned to the right
 * edge even when `left` is omitted (the left cell is rendered empty), so the
 * forward action keeps a stable position across every step.
 */
function WizardFooter(props: { left?: ReactNode; right: ReactNode }): JSX.Element {
  const { left, right } = props;
  return (
    <div className="flex items-center justify-between gap-3 border-t border-border pt-4">
      <div>{left}</div>
      <div>{right}</div>
    </div>
  );
}

export function BatchImportWizard(props: {
  /**
   * Runs the feature's import mutation. Throw on transport error (surfaces as a
   * banner); return null on missing data (surfaces the unexpectedError banner).
   */
  onImport: (payload: string) => Promise<ImportResult | null>;
  /** The wrapper's `useMutation` loading flag, passed through unchanged. */
  importing: boolean;
  /** Connection document to refetch after a successful import. */
  refetchDocument: DocumentNode;
  /** Logged in the refetch-failure warn (debug context only). */
  targetId: string;
  /** Name shown in the step-2 confirm heading (deck / cardgroup name). */
  targetName: string;
  onImported?: () => void;
  onCancel?: () => void;
}): JSX.Element {
  const { onImport, importing, refetchDocument, targetId, targetName, onImported, onCancel } =
    props;
  const t = useTranslations("BatchImport");

  const [step, setStep] = useState<1 | 2>(1);
  const [payloadText, setPayloadText] = useState<string>("");
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null);
  // validatedPayload tracks the payloadText value that was in effect when the
  // last successful validate call completed. resolveStep1Button compares this
  // against the current payloadText (the isStale flag) to prevent continuing to
  // import a stale/edited payload without re-validating. It is also cleared after
  // each successful import attempt so a re-import requires re-validation.
  const [validatedPayload, setValidatedPayload] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [bannerError, setBannerError] = useState<string>("");

  const apollo = useApolloClient();

  const [runValidate, { loading: validating }] = useLazyQuery(ValidateCardImportQuery, {
    fetchPolicy: "no-cache",
  });

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
        // Record which payloadText was validated so resolveStep1Button can detect stale edits.
        setValidatedPayload(payloadText);
      }
      if (result.error) {
        setBannerError(classifyError(result.error, t("unexpectedError")));
      }
    } catch (err) {
      setBannerError(classifyError(err, t("unexpectedError")));
    }
  }

  async function handleImport() {
    setBannerError("");
    setImportResult(null);
    const payload = encodePayload(payloadText);
    try {
      const data = await onImport(payload);
      if (!data) {
        setBannerError(t("unexpectedError"));
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
        await apollo.refetchQueries({ include: [refetchDocument] });
      } catch (refetchErr) {
        // A failed refetch must not block closing the sheet on a successful
        // import; the next mount/poll re-reads the list. Log and continue.
        console.warn("[BatchImportWizard] cards refetch failed", {
          name: refetchErr instanceof Error ? refetchErr.name : "unknown",
          targetId,
        });
      }
      if (data.errors.length === 0) {
        // Full success: close the sheet.
        onImported?.();
      }
      // Partial failure (error rows present): keep the sheet open so the result
      // banner and error rows stay visible.
    } catch (err) {
      setBannerError(classifyError(err, t("unexpectedError")));
    }
  }

  const parsedCards = validationResult?.parsedCards ?? [];
  // Duplicate-front rows are non-fatal warnings (last-write-wins still
  // persisted the row); every other kind is a genuine error.
  const importWarningCount = importResult?.errors.filter((e) => isWarningKind(e.kind)).length ?? 0;
  const importErrorCount = importResult?.errors.filter((e) => !isWarningKind(e.kind)).length ?? 0;
  // "Failed" is keyed on genuine errors only: warnings never block a row from
  // persisting, so a result with nothing persisted and only warnings is still
  // a success (and in practice cannot occur — a warning implies a winner row).
  const importAllFailed =
    importResult != null &&
    importResult.inserted === 0 &&
    importResult.updated === 0 &&
    importErrorCount > 0;

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
    } else if (buttonSpec.action === "continue") {
      setImportResult(null);
      setBannerError("");
      setStep(2);
    }
  }

  return (
    <div className="space-y-6">
      <ImportStepper current={step} importing={importing} onBack={goBackToStep1} />

      {bannerError && <ErrorBanner>{bannerError}</ErrorBanner>}

      {step === 1 ? (
        <div className="space-y-4">
          <div className="space-y-2">
            <label htmlFor="batch-import-payload" className="block text-xs">
              <span className="font-medium text-muted-foreground">{t("cardsToImport")}</span>
              <span className="font-normal text-muted-foreground/70">{t("tabHint")}</span>
            </label>
            <Textarea
              id="batch-import-payload"
              data-testid="batch-import-payload"
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
              data-testid="batch-import-validate-status"
            >
              {validationResult.valid
                ? t("validResult", { count: parsedCards.length })
                : t("invalidResult", { count: validationResult.errors.length })}
            </div>
          )}

          {validationResult && <ValidateResult result={validationResult} />}

          <WizardFooter
            left={
              <Button type="button" variant="ghost" onClick={() => onCancel?.()}>
                {t("backToCardgroup")}
              </Button>
            }
            right={
              <Button
                type="button"
                variant={buttonSpec.action !== null ? "brand" : "outline"}
                onClick={onStep1ButtonClick}
                disabled={buttonSpec.disabled}
                data-testid="batch-import-step1-btn"
              >
                {t(buttonSpec.labelKey)}
              </Button>
            }
          />
        </div>
      ) : (
        <div className="space-y-4">
          {importResult ? (
            <section className="space-y-4">
              <div role="status">
                {importAllFailed ? (
                  <ErrorBanner>{t("importFailed", { count: importErrorCount })}</ErrorBanner>
                ) : (
                  <div className="rounded-md bg-green-50 p-3 text-sm text-green-800">
                    {t("importComplete", {
                      inserted: importResult.inserted,
                      updated: importResult.updated,
                    })}
                    {importWarningCount > 0 && t("withWarnings", { count: importWarningCount })}
                    {importErrorCount > 0 && t("withErrors", { count: importErrorCount })}
                    {"."}
                  </div>
                )}
              </div>
              {importResult.errors.length > 0 && <ErrorList errors={importResult.errors} />}
              <WizardFooter
                left={
                  <Button type="button" variant="outline" onClick={goBackToStep1}>
                    {t("backToEdit")}
                  </Button>
                }
                right={
                  <Button type="button" variant="brand" onClick={() => onImported?.()}>
                    {t("done")}
                  </Button>
                }
              />
            </section>
          ) : (
            <>
              <h2 className="text-base font-semibold">
                {t("confirmHeading", { count: parsedCards.length, cardgroupName: targetName })}
              </h2>
              <p className="text-sm text-muted-foreground">{t("confirmDesc")}</p>
              <WizardFooter
                left={
                  <Button
                    type="button"
                    variant="outline"
                    onClick={goBackToStep1}
                    disabled={importing}
                  >
                    {t("backToEdit")}
                  </Button>
                }
                right={
                  <Button
                    type="button"
                    variant="brand"
                    onClick={() => {
                      void handleImport();
                    }}
                    disabled={importing}
                    data-testid="batch-import-confirm-btn"
                  >
                    {importing ? t("importing") : t("importButton", { count: parsedCards.length })}
                  </Button>
                }
              />
            </>
          )}
        </div>
      )}
    </div>
  );
}
