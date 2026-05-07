import { List, Pencil, Play, Plus } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { CardgroupQuery, CardsByCardgroupQuery } from "@/app/cardgroups/queries";
import { Button } from "@/components/ui/button";
import type {
  CardgroupQuery as CardgroupQueryType,
  CardsByCardgroupQuery as CardsByCardgroupQueryType,
} from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { formatMediumDate } from "@/lib/format";
import { createSupabaseServerClient } from "@/lib/supabase/server";

function truncateFront(front: string): string {
  if (front.length > 80) return `${front.slice(0, 79)}…`;
  return front;
}

export default async function CardgroupDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cardgroups/:id] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  const { id } = await params;

  let cardgroupData: CardgroupQueryType | null = null;
  let cardsData: CardsByCardgroupQueryType | null = null;

  try {
    [cardgroupData, cardsData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id }, revalidate: 0 }),
      gqlFetch(CardsByCardgroupQuery, { variables: { cardgroupId: id }, revalidate: 0 }),
    ]);
  } catch (err) {
    redirectIfUnauthenticated(err, "/cardgroups");
  }

  if (!cardgroupData?.cardgroup) redirect("/cardgroups");

  const cardgroup = cardgroupData.cardgroup;
  const cards = cardsData?.cardsByCardgroup ?? [];
  const previewCards = cards.slice(0, 5);

  return (
    <main className="p-8">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-2">
        <div>
          <h1 className="mb-2 text-2xl font-semibold">{cardgroup.name}</h1>
          <p className="text-sm text-muted-foreground">
            Updated {formatMediumDate(cardgroup.updatedAt as string)}
          </p>
        </div>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <Link href={`/cardgroups/${id}/edit`}>
              <span>Edit</span>
              <Pencil aria-hidden="true" />
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link href={`/cardgroups/${id}/cards`}>
              <span>Manage cards</span>
              <List aria-hidden="true" />
            </Link>
          </Button>
          <Button asChild variant="brand">
            <Link href={`/learn/${id}`}>
              <span>Start learning</span>
              <Play aria-hidden="true" />
            </Link>
          </Button>
        </div>
      </div>

      {cards.length === 0 ? (
        <div className="mb-8 rounded-lg border border-dashed border-border p-6 text-center">
          <p className="mb-4 text-muted-foreground">No cards yet. Add some to get started.</p>
          {/* Hidden on mobile; GlobalFAB reaches the same end goal (open the
              card creation form for this cardgroup) via the card-with-group
              FAB variant, which routes directly to /cards/new?cardgroup=<id>. */}
          <Button asChild variant="brand" className="hidden md:inline-flex">
            <Link href={`/cardgroups/${id}/cards`}>
              <span>Add card</span>
              <Plus aria-hidden="true" />
            </Link>
          </Button>
        </div>
      ) : (
        <section className="mb-8">
          <h2 className="mb-3 text-sm font-medium text-muted-foreground uppercase tracking-wide">
            Preview
          </h2>
          <ul className="space-y-2">
            {previewCards.map((card) => (
              <li
                key={card.id}
                className="rounded-md border border-border bg-card px-4 py-2 text-sm"
              >
                {truncateFront(card.front)}
              </li>
            ))}
          </ul>
          {cards.length > 5 && (
            <p className="mt-2 text-xs text-muted-foreground">Showing 5 of {cards.length}</p>
          )}
        </section>
      )}
    </main>
  );
}
