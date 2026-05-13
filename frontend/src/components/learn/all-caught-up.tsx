import Link from "next/link";
import { Button } from "@/components/ui/button";

export function AllCaughtUp() {
  return (
    <section className="flex flex-1 items-center justify-center">
      <div className="w-full max-w-md rounded-lg border border-dashed border-border p-8 text-center">
        <h1 className="mb-2 text-xl font-semibold">Today's learning is complete</h1>
        <p className="mb-6 text-sm text-muted-foreground">
          All cards for this group have been reviewed. Come back when the next review is due.
        </p>
        <Button asChild variant="brand">
          <Link href="/cardgroups">Back to cardgroups</Link>
        </Button>
      </div>
    </section>
  );
}
