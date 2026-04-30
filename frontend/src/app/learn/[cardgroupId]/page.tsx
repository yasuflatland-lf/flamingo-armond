import Link from "next/link";
import { redirect } from "next/navigation";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  LearnCardsByCardgroupQuery as LearnCardsByCardgroupQueryType,
} from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LearnCardsByCardgroupQuery } from "../queries";
import { LearnClient } from "./learn-client";

export default async function LearnPage({ params }: { params: Promise<{ cardgroupId: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  if (!user) redirect("/login");
  if (authErr) throw authErr;

  const { cardgroupId } = await params;

  let cardgroupData: CardgroupQueryType | null = null;
  let cardsData: LearnCardsByCardgroupQueryType | null = null;

  try {
    [cardgroupData, cardsData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id: cardgroupId }, revalidate: 0 }),
      gqlFetch(LearnCardsByCardgroupQuery, { variables: { cardgroupId }, revalidate: 0 }),
    ]);
  } catch (err) {
    redirectIfUnauthenticated(err, "/cardgroups");
  }

  if (!cardgroupData?.cardgroup) redirect("/cardgroups");

  const cards = cardsData?.cardsByCardgroup ?? [];

  return (
    <main className="min-h-[calc(100vh-4rem)] bg-background">
      <div className="mx-auto flex min-h-[calc(100vh-4rem)] w-full max-w-5xl flex-col px-4 py-6 sm:px-6 lg:px-8">
        <div className="mb-4 flex items-center justify-between gap-4">
          <Link
            href={`/cardgroups/${cardgroupId}`}
            className="text-sm text-muted-foreground hover:underline"
          >
            Back
          </Link>
          <p className="truncate text-sm font-medium text-muted-foreground">
            {cardgroupData.cardgroup.name}
          </p>
        </div>
        <LearnClient cardgroupId={cardgroupId} initialCards={cards} />
      </div>
    </main>
  );
}
