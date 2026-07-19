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

/** Exact share of new cards a stored (possibly reduced) fraction stands for. */
function exactPercent(numerator: number, denominator: number): number {
  return (numerator / denominator) * 100;
}

// Snap a percent onto the 5% grid, clamped to [MIN, MAX]. `updateNewCardRatio`
// accepts any reduced fraction with 1 <= numerator < denominator <= 100, so an
// off-grid ratio (33/100) is a legitimate stored state; the snap only positions
// the 5%-step control and never stands in for the stored value.
function snapPercent(percent: number): number {
  return Math.min(MAX, Math.max(MIN, Math.round(percent / STEP) * STEP));
}

/** At most one decimal, so 1/3 reads 33.3% while 4/5 stays 80%. */
function roundForDisplay(percent: number): number {
  return Math.round(percent * 10) / 10;
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
 * The labels report the exact stored ratio, not the slider's grid position, so
 * an off-grid value set through the API stays visible; a note discloses that
 * moving the slider overwrites it with a 5% step.
 *
 * This component assumes it is only mounted for admins — the parent gates on
 * `isAdmin`.
 */
export function NewCardRatioSection({ initialRatio }: Props) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
  // Slider position: always on the 5% grid.
  const [percent, setPercent] = useState(() =>
    snapPercent(exactPercent(initialRatio.numerator, initialRatio.denominator)),
  );
  // Last server-confirmed value, exact rather than snapped: what the labels
  // show whenever the slider is at rest.
  const [confirmedPercent, setConfirmedPercent] = useState(() =>
    exactPercent(initialRatio.numerator, initialRatio.denominator),
  );
  // True while the user drags or a save is in flight, when the labels must
  // track the slider instead of the not-yet-replaced server-confirmed value.
  const [trackingSlider, setTrackingSlider] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [updateRatio, { loading }] = useMutation(UpdateNewCardRatioMutation);
  // Rollback target after intermediate onValueChange updates; a ref so a commit
  // reads the latest confirmed value without waiting for a re-render.
  const confirmedRef = useRef(confirmedPercent);

  async function handleCommit(next: number) {
    if (loading) return;
    if (next === confirmedRef.current) {
      setTrackingSlider(false);
      return;
    }
    const previous = confirmedRef.current;
    setSaveError(null);
    setPercent(next);
    try {
      await updateRatio({ variables: { numerator: next, denominator: 100 } });
      confirmedRef.current = next;
      setConfirmedPercent(next);
    } catch (err) {
      // Roll the slider back to the last server-confirmed value so the UI never
      // shows a ratio the server rejected.
      setPercent(snapPercent(previous));
      setSaveError(
        mutationAuthBanner(err, {
          forbidden: tCommon("forbidden"),
          unauthenticated: t("sessionExpired"),
          fallback: tCommon("somethingWentWrong"),
        }),
      );
      console.warn("[profile] updateNewCardRatio rejected", { codes: liftGraphQLCodes(err) });
    } finally {
      setTrackingSlider(false);
    }
  }

  const displayPercent = trackingSlider ? percent : confirmedPercent;
  const isOffGrid = confirmedPercent !== snapPercent(confirmedPercent);

  return (
    <fieldset className="flex flex-col gap-2 border-0 p-0">
      <legend className="mb-2 text-sm font-medium leading-none">{t("newCardRatio")}</legend>
      <div className="flex max-w-xs items-baseline justify-between">
        <span className="text-sm font-semibold text-brand-link">
          {t("newCardRatioNewLabel", { percent: roundForDisplay(displayPercent) })}
        </span>
        <span className="text-sm text-muted-foreground">
          {t("newCardRatioReviewLabel", { percent: roundForDisplay(100 - displayPercent) })}
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
        onValueChange={([next = percent]) => {
          setTrackingSlider(true);
          setPercent(next);
        }}
        onValueCommit={([next = percent]) => handleCommit(next)}
        data-testid="new-card-ratio-slider"
      />
      <div className="flex max-w-xs justify-between text-xs text-muted-foreground">
        <span>{MIN}%</span>
        <span>{MAX}%</span>
      </div>
      {isOffGrid ? (
        <p
          className="max-w-xs text-xs text-muted-foreground"
          data-testid="new-card-ratio-custom-notice"
        >
          {t("newCardRatioCustomNotice", { percent: roundForDisplay(confirmedPercent) })}
        </p>
      ) : null}
      {saveError ? <ErrorBanner data-testid="new-card-ratio-error">{saveError}</ErrorBanner> : null}
    </fieldset>
  );
}
