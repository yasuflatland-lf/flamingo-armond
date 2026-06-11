"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

type Props = {
  /** Heading copy. Falls back to the `Learn.completeHeading` message when omitted. */
  heading?: string;
  /** Body copy. Falls back to the `Learn.completeMessage` message when omitted. */
  message?: string;
  /**
   * When provided, render a primary "Study again" button above the always-present
   * "Back to cardgroups" link. Used to offer FSRS-safe practice mode (re-study
   * today's already-reviewed cards) or to restart a finished practice round.
   */
  onStudyAgain?: () => void;
};

export function AllCaughtUp({ heading, message, onStudyAgain }: Props) {
  const t = useTranslations("Learn");
  return (
    <section className="flex flex-1 items-center justify-center">
      <div className="w-full max-w-md rounded-lg border border-dashed border-border p-8 text-center">
        <h1 className="mb-2 text-xl font-semibold">{heading ?? t("completeHeading")}</h1>
        <p className="mb-6 text-sm text-muted-foreground">{message ?? t("completeMessage")}</p>
        <div className="flex flex-col items-center gap-3">
          {onStudyAgain ? (
            <Button type="button" variant="brand" onClick={onStudyAgain}>
              {t("studyAgain")}
            </Button>
          ) : null}
          <Button asChild variant={onStudyAgain ? "outline" : "brand"}>
            <Link href="/cardgroups">{t("backToCardgroups")}</Link>
          </Button>
        </div>
      </div>
    </section>
  );
}
