"use client";

import { useMutation } from "@apollo/client/react";
import { useTranslations } from "next-intl";
import { useId, useRef, useState } from "react";
import { UpdateNewCardRatioMutation } from "@/app/learn/queries";
import { ErrorBanner } from "@/components/ui/error-banner";
import { Slider } from "@/components/ui/slider";
import { mutationAuthBanner } from "@/lib/apollo/errors";
import { liftGraphQLCodes } from "@/lib/apollo/graphql-errors";

const MIN = 5;
const MAX = 95;
const STEP = 5;

type Ratio = { numerator: number; denominator: number };

// `updateNewCardRatio` accepts any reduced fraction with
// 1 <= numerator < denominator <= 100, so an off-grid ratio (33/100) is a
// legitimate stored state. That stored value IS a rational number, and every
// question this component asks about it — is it on the 5% grid, where does the
// control sit, what do the labels read — has an exact integer answer, so the
// helpers below scale to integers before dividing instead of dividing first and
// rounding the IEEE-754 error away afterwards.

/**
 * Exact share of new cards in tenths of a percent (0..1000), round-half-up:
 * 1000n/d rounded is `floor((2000n + d) / 2d)`. Every term is a safe integer
 * because 2000n <= 200000. JS has no integer division, so the quotient is a
 * float — but the `Math.floor` is still exact: a non-integral quotient of these
 * operands sits at least 1/2d away from any integer, and d <= 100 puts that at
 * 0.005 or more, many orders of magnitude beyond double rounding error. An
 * integral quotient divides exactly.
 */
function shareTenths({ numerator, denominator }: Ratio): number {
  return Math.floor((2000 * numerator + denominator) / (2 * denominator));
}

/**
 * True when the exact share 100n/d is a whole multiple of 5, which happens
 * exactly when d divides 20n. n < d keeps the share strictly inside (0, 100),
 * so an on-grid share always lands within [MIN, MAX] and the clamp never
 * interacts with this test.
 */
function isOnGrid({ numerator, denominator }: Ratio): boolean {
  return (20 * numerator) % denominator === 0;
}

/**
 * Position of the 5%-step control: the nearest grid step to the exact share,
 * clamped to [MIN, MAX]. The grid index is 20n/d rounded half-up, i.e.
 * `floor((40n + d) / 2d)`. Its `Math.floor` is exact for the same reason as
 * `shareTenths`: these operands put every non-integral quotient at least
 * 1/2d >= 0.005 clear of the nearest integer. The control only positions itself
 * here — it never stands in for the stored value.
 */
function gridPercent({ numerator, denominator }: Ratio): number {
  const gridIndex = Math.floor((40 * numerator + denominator) / (2 * denominator));
  return Math.min(MAX, Math.max(MIN, STEP * gridIndex));
}

/** True when the stored ratio is exactly `percent`%, i.e. 100n/d === percent. */
function equalsPercent({ numerator, denominator }: Ratio, percent: number): boolean {
  return 100 * numerator === percent * denominator;
}

/**
 * The number the ICU message interpolates: 800 -> 80, 333 -> 33.3. Dividing a
 * tenths integer by 10 yields the only float that reaches the UI, and it is
 * exact in the only sense that matters here — the quotient is the double
 * nearest that decimal, so it prints back as the same one-decimal string.
 */
function tenthsToPercent(tenths: number): number {
  return tenths / 10;
}

type Props = { initialRatio: Ratio };

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
 * moving the slider overwrites it with a 5% step, and `aria-describedby` ties
 * that note to the thumb so a screen reader reaches the stored value too —
 * `aria-valuetext` stays faithful to where the control actually sits.
 *
 * This component assumes it is only mounted for admins — the parent gates on
 * `isAdmin`.
 */
export function NewCardRatioSection({ initialRatio }: Props) {
  const t = useTranslations("Profile");
  const tCommon = useTranslations("Common");
  const noticeId = useId();
  // Slider position: always on the 5% grid.
  const [percent, setPercent] = useState(() => gridPercent(initialRatio));
  // Last server-confirmed ratio, kept as the stored fraction rather than a
  // percent: it is what the labels report whenever the slider is at rest, and
  // the exact-integer helpers all read a fraction.
  const [confirmedRatio, setConfirmedRatio] = useState<Ratio>(initialRatio);
  // True while the user drags or a save is in flight, when the labels must
  // track the slider instead of the not-yet-replaced server-confirmed value.
  const [trackingSlider, setTrackingSlider] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [updateRatio, { loading }] = useMutation(UpdateNewCardRatioMutation);
  // Rollback target after intermediate onValueChange updates; a ref so a commit
  // reads the latest confirmed value without waiting for a re-render.
  const confirmedRef = useRef(confirmedRatio);

  async function handleCommit(next: number) {
    if (loading) return;
    if (equalsPercent(confirmedRef.current, next)) {
      setTrackingSlider(false);
      return;
    }
    const previous = confirmedRef.current;
    setSaveError(null);
    setPercent(next);
    try {
      await updateRatio({ variables: { numerator: next, denominator: 100 } });
      const committed = { numerator: next, denominator: 100 };
      confirmedRef.current = committed;
      setConfirmedRatio(committed);
    } catch (err) {
      // Roll the slider back to the last server-confirmed value so the UI never
      // shows a ratio the server rejected.
      setPercent(gridPercent(previous));
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

  // A dragged value is a whole percent, so it is already an exact tenths count.
  const displayTenths = trackingSlider ? percent * 10 : shareTenths(confirmedRatio);
  const displayNewPercent = tenthsToPercent(displayTenths);
  // The review side is the complement of the same tenths count, so the pair
  // sums to exactly 100.0 by construction — rounding each side on its own would
  // print 6.3% / 93.8% for an exact share landing on a .x5 boundary (1/16).
  const displayReviewPercent = tenthsToPercent(1000 - displayTenths);
  // The stored share only prints as a grid value when it IS one: an off-grid
  // share sits at least 5/99 of a percentage point from the grid, further than
  // the tenths rounding can travel.
  const noticePercent = tenthsToPercent(shareTenths(confirmedRatio));
  const isOffGrid = !isOnGrid(confirmedRatio);

  return (
    <fieldset className="flex flex-col gap-2 border-0 p-0">
      <legend className="mb-2 text-sm font-medium leading-none">{t("newCardRatio")}</legend>
      <div className="flex max-w-xs items-baseline justify-between">
        <span className="text-sm font-semibold text-brand-link">
          {t("newCardRatioNewLabel", { percent: displayNewPercent })}
        </span>
        <span className="text-sm text-muted-foreground">
          {t("newCardRatioReviewLabel", { percent: displayReviewPercent })}
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
        aria-describedby={isOffGrid ? noticeId : undefined}
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
          id={noticeId}
          className="max-w-xs text-xs text-muted-foreground"
          data-testid="new-card-ratio-custom-notice"
        >
          {t("newCardRatioCustomNotice", { percent: noticePercent })}
        </p>
      ) : null}
      {saveError ? <ErrorBanner data-testid="new-card-ratio-error">{saveError}</ErrorBanner> : null}
    </fieldset>
  );
}
