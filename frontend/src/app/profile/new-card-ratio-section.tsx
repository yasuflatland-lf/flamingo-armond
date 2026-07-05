"use client";

import { useMutation } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useRef, useState } from "react";
import { UpdateNewCardRatioMutation } from "@/app/learn/queries";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Slider } from "@/components/ui/slider";
import { mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

const MIN = 5;
const MAX = 95;
const STEP = 5;

// Snap a stored (possibly reduced) fraction onto the 5% grid, clamped to
// [MIN, MAX]. The default 4/5 -> 80 is already on-grid; the snap is defensive
// for legacy rows.
function snapPercent(numerator: number, denominator: number): number {
  const raw = (numerator / denominator) * 100;
  return Math.min(MAX, Math.max(MIN, Math.round(raw / STEP) * STEP));
}

type Props = { initialRatio: { numerator: number; denominator: number } };

/**
 * New-card ratio slider on `/profile`: lets the signed-in user pick what share
 * of a learn session is never-seen cards vs. reviews. 5% steps, new 5%-95%.
 * Auto-saves on release via `updateNewCardRatio` (numerator = percent,
 * denominator = 100; the backend reduces). Optimistic local state, rolled back
 * on failure. No Apollo `optimisticResponse`: the mutation can return
 * UNAUTHENTICATED and Apollo v3 does not reliably roll those back.
 *
 * This component assumes it is only mounted for admins — the parent gates on
 * `isAdmin`.
 */
export function NewCardRatioSection({ initialRatio }: Props) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
  const [percent, setPercent] = useState(() =>
    snapPercent(initialRatio.numerator, initialRatio.denominator),
  );
  const [saveError, setSaveError] = useState<string | null>(null);
  const [updateRatio, { loading }] = useMutation(UpdateNewCardRatioMutation);
  // Last server-confirmed value; rollback target after intermediate
  // onValueChange updates.
  const committedRef = useRef(percent);

  async function handleCommit(next: number) {
    if (next === committedRef.current || loading) return;
    const previous = committedRef.current;
    setSaveError(null);
    setPercent(next);
    try {
      await updateRatio({ variables: { numerator: next, denominator: 100 } });
      committedRef.current = next;
    } catch (err) {
      // Roll the slider back to the last server-confirmed value so the UI never
      // shows a ratio the server rejected.
      setPercent(previous);
      setSaveError(
        mutationAuthBanner(err, {
          forbidden: tCommon("forbidden"),
          unauthenticated: t("sessionExpired"),
          fallback: tCommon("somethingWentWrong"),
        }),
      );
      console.warn("[profile] updateNewCardRatio rejected", { codes: liftGraphQLCodes(err) });
    }
  }

  return (
    <fieldset className="flex flex-col gap-2 border-0 p-0">
      <legend className="mb-2 text-sm font-medium leading-none">{t("newCardRatio")}</legend>
      <div className="flex max-w-xs items-baseline justify-between">
        <span className="text-sm font-semibold text-brand-link">
          {t("newCardRatioNewLabel", { percent })}
        </span>
        <span className="text-sm text-muted-foreground">
          {t("newCardRatioReviewLabel", { percent: 100 - percent })}
        </span>
      </div>
      <Slider
        className="max-w-xs"
        min={MIN}
        max={MAX}
        step={STEP}
        value={[percent]}
        disabled={loading}
        aria-label={t("newCardRatio")}
        aria-valuetext={t("newCardRatioValueText", { newPct: percent, reviewPct: 100 - percent })}
        onValueChange={([next = percent]) => setPercent(next)}
        onValueCommit={([next = percent]) => handleCommit(next)}
        data-testid="new-card-ratio-slider"
      />
      <div className="flex max-w-xs justify-between text-xs text-muted-foreground">
        <span>{MIN}%</span>
        <span>{MAX}%</span>
      </div>
      {saveError ? <ErrorBanner data-testid="new-card-ratio-error">{saveError}</ErrorBanner> : null}
    </fieldset>
  );
}
