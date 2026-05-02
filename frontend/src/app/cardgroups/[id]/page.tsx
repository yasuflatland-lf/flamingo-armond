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
    <main className="mx-auto max-w-2xl p-8">
      <h1 className="mb-2 text-2xl font-semibold">{cardgroup.name}</h1>
      <p className="mb-6 text-sm text-muted-foreground">
        Updated {formatMediumDate(cardgroup.updatedAt as string)}
      </p>

      {cards.length === 0 ? (
        <div className="mb-8 rounded-lg border border-dashed border-border p-6 text-center">
          <p className="mb-4 text-muted-foreground">No cards yet. Add some to get started.</p>
          <Button asChild>
            <Link href={`/cardgroups/${id}/cards`}>Add card</Link>
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

      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link href={`/learn/${id}`}>Start learning</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href={`/cardgroups/${id}/edit`}>Edit</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href={`/cardgroups/${id}/cards`}>Manage cards</Link>
        </Button>
      </div>
    </main>
  );
}
