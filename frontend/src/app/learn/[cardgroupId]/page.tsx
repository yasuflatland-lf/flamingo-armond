import { redirect } from "next/navigation";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import { MeWithLastViewedQuery } from "@/app/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  LearnCardsByCardgroupQuery as LearnCardsByCardgroupQueryType,
  MeWithLastViewedQuery as MeWithLastViewedQueryType,
} from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { LearnCardsByCardgroupQuery } from "../queries";
import { LearnClient } from "./learn-client";

export default async function LearnPage({ params }: { params: Promise<{ cardgroupId: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[learn] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  const { cardgroupId } = await params;

  let cardgroupData: CardgroupQueryType;
  let cardsData: LearnCardsByCardgroupQueryType;
  let meData: MeWithLastViewedQueryType;

  try {
    [cardgroupData, cardsData, meData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id: cardgroupId }, revalidate: 0 }),
      gqlFetch(LearnCardsByCardgroupQuery, { variables: { cardgroupId }, revalidate: 0 }),
      gqlFetch(MeWithLastViewedQuery, { revalidate: 0 }),
    ]);
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/cardgroups");
    }
    console.error("[learn] gqlFetch batch failed:", err);
    throw err;
  }

  if (!cardgroupData.cardgroup) redirect("/cardgroups");

  const cards = cardsData.cardsByCardgroup ?? [];
  const lastViewedCardgroupId = meData.me?.lastViewedCardgroup?.id ?? null;

  return (
    <main className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col p-4">
        <LearnClient
          cardgroupId={cardgroupId}
          cardgroupName={cardgroupData.cardgroup.name}
          initialCards={cards}
          lastViewedCardgroupId={lastViewedCardgroupId}
        />
      </div>
    </main>
  );
}
