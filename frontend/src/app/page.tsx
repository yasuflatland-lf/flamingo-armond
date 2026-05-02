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

  if (authErr) {
    if (authErr.name === "AuthSessionMissingError") {
      redirect("/login");
    }
    console.error("[home] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }

  if (!user) {
    redirect("/login");
  }

  let data: MeWithLastViewedQueryType | null = null;
  try {
    data = await gqlFetch(MeWithLastViewedQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[home] gqlFetch failed:", err);
    throw err;
  }

  // `data` is non-null here: the catch block always redirects or rethrows, so
  // the try block either populated `data` or threw (then redirected/rethrew).
  if (!data) {
    throw new Error("[home] unreachable: gqlFetch resolved without data");
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
