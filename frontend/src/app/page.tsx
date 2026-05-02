import { redirect } from "next/navigation";
import { MeWithLastViewedQuery } from "@/app/queries";
import type { MeWithLastViewedQuery as MeWithLastViewedQueryType } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";

/**
 * Root redirect for the app.
 *
 *   1. supabase.auth.getUser()
 *      - AuthSessionMissingError (no session) → /login
 *      - other error                          → console.error → throw
 *
 *   2. gqlFetch(MeWithLastViewedQuery, { revalidate: 0 })
 *      - UNAUTHENTICATED → /login (via redirectIfUnauthenticated)
 *      - other error      → throw
 *
 *   3. Branch on the result:
 *      a) me.lastViewedCardgroup != null → /learn/{id}
 *      b) myCardgroups not empty         → /cardgroups
 *      c) otherwise                       → /cardgroups/new?welcome=1
 */
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
    redirectIfUnauthenticated(err, "/login");
  }

  // `data` is non-null here: redirectIfUnauthenticated returns `never`, and the
  // try block either populated `data` or threw (then redirected/rethrew).
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
