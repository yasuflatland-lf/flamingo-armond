import { redirect } from "next/navigation";
import { MeWithLastViewedQuery } from "@/app/queries";
import type { MeWithLastViewedQuery as MeWithLastViewedQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";

// Root redirect — see docs/frontend.md § routing topology.
export default async function HomePage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();

  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[home] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  let data: MeWithLastViewedQueryType;
  try {
    data = await gqlFetch(MeWithLastViewedQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[home] gqlFetch failed:", err);
    throw err;
  }

  const lastViewedId = data.me?.lastViewedCardgroup?.id ?? null;
  if (lastViewedId) {
    redirect(`/learn/${lastViewedId}`);
  }

  if (data.myCardgroups.length > 0) {
    redirect("/cardgroups");
  }

  redirect("/cardgroups/new?welcome=1");
}
