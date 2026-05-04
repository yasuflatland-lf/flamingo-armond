import Link from "next/link";
import { redirect } from "next/navigation";
import { CardgroupQuery } from "@/app/cardgroups/queries";
import type {
  CardgroupQuery as CardgroupQueryType,
  CardsByCardgroupConnectionQuery as CardsByCardgroupConnectionQueryType,
} from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { CardsClient } from "./cards-client";
import { CARDS_PAGE_SIZE, CardsByCardgroupConnectionQuery } from "./queries";

export default async function CardsPage({ params }: { params: Promise<{ id: string }> }) {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cardgroups/:id/cards] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  const { id } = await params;

  let cardgroupData: CardgroupQueryType | null = null;
  let connectionData: CardsByCardgroupConnectionQueryType | null = null;

  try {
    [cardgroupData, connectionData] = await Promise.all([
      gqlFetch(CardgroupQuery, { variables: { id }, revalidate: 0 }),
      gqlFetch(CardsByCardgroupConnectionQuery, {
        variables: { cardgroupId: id, first: CARDS_PAGE_SIZE },
        revalidate: 0,
      }),
    ]);
  } catch (err) {
    redirectIfUnauthenticated(err, "/cardgroups");
  }

  if (!cardgroupData?.cardgroup) redirect("/cardgroups");

  const cardgroup = cardgroupData.cardgroup;
  const initialEdges = connectionData?.cardsByCardgroupConnection.edges ?? [];
  const initialPageInfo = connectionData?.cardsByCardgroupConnection.pageInfo ?? {
    hasNextPage: false,
    hasPreviousPage: false,
    startCursor: null,
    endCursor: null,
  };
  const initialTotalCount = connectionData?.cardsByCardgroupConnection.totalCount ?? 0;

  return (
    <main className="mx-auto max-w-2xl p-8">
      <div className="mb-6 flex items-center gap-4">
        <Link href={`/cardgroups/${id}`} className="text-sm text-muted-foreground hover:underline">
          &larr; Back
        </Link>
      </div>
      <h1 className="mb-6 text-2xl font-semibold">Cards in {cardgroup.name}</h1>
      <CardsClient
        cardgroupId={id}
        initialEdges={initialEdges}
        initialPageInfo={initialPageInfo}
        initialTotalCount={initialTotalCount}
      />
    </main>
  );
}
