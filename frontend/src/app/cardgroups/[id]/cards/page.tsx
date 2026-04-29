import Link from "next/link";
import { redirect } from "next/navigation";
import { CardgroupQuery, CardsByCardgroupQuery } from "@/app/cardgroups/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  CardsByCardgroupQuery as CardsByCardgroupQueryType,
} from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { CardsClient } from "./cards-client";

export default async function CardsPage({ params }: { params: Promise<{ id: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (authErr) throw authErr;
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
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("UNAUTHENTICATED")) redirect("/cardgroups");
    throw err;
  }

  if (!cardgroupData?.cardgroup) redirect("/cardgroups");

  const cardgroup = cardgroupData.cardgroup;
  const cards = cardsData?.cardsByCardgroup ?? [];

  return (
    <main className="mx-auto max-w-2xl p-8">
      <div className="mb-6 flex items-center gap-4">
        <Link href={`/cardgroups/${id}`} className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
      </div>
      <h1 className="mb-6 text-2xl font-semibold">Cards in {cardgroup.name}</h1>
      <CardsClient cardgroupId={id} initialCards={cards} />
    </main>
  );
}
