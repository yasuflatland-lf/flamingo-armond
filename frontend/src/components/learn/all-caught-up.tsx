import Link from "next/link";
import { Button } from "@/components/ui/button";

const DEFAULT_HEADING = "Today's learning is complete";
const DEFAULT_MESSAGE =
  "All cards for this group have been reviewed. Come back when the next review is due.";

type Props = {
  /** Heading copy. Defaults to the end-of-daily-learn message. */
  heading?: string;
  /** Body copy. Defaults to the end-of-daily-learn message. */
  message?: string;
  /**
   * When provided, render a primary "Study again" button above the always-present
   * "Back to cardgroups" link. Used to offer FSRS-safe practice mode (re-study
   * today's already-reviewed cards) or to restart a finished practice round.
   */
  onStudyAgain?: () => void;
};

export function AllCaughtUp({
  heading = DEFAULT_HEADING,
  message = DEFAULT_MESSAGE,
  onStudyAgain,
}: Props) {
  return (
    <section className="flex flex-1 items-center justify-center">
      <div className="w-full max-w-md rounded-lg border border-dashed border-border p-8 text-center">
        <h1 className="mb-2 text-xl font-semibold">{heading}</h1>
        <p className="mb-6 text-sm text-muted-foreground">{message}</p>
        <div className="flex flex-col items-center gap-3">
          {onStudyAgain ? (
            <Button type="button" variant="brand" onClick={onStudyAgain}>
              Study again
            </Button>
          ) : null}
          <Button asChild variant={onStudyAgain ? "outline" : "brand"}>
            <Link href="/cardgroups">Back to cardgroups</Link>
          </Button>
        </div>
      </div>
    </section>
  );
}
