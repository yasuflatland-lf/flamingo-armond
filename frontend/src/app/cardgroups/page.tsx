import { redirect } from "next/navigation";
import type { MyCardgroupsConnectionQuery as MyCardgroupsConnectionQueryType } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { redirectIfUnauthenticated } from "@/lib/apollo/server-redirect";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import CardgroupsClient from "./cardgroups-client";
import { CARDGROUPS_PAGE_SIZE, MyCardgroupsConnectionQuery } from "./queries";

type CardgroupConnection = MyCardgroupsConnectionQueryType["myCardgroupsConnection"];

export default async function CardgroupsPage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError is the "no session" signal — fall through to the
  // !user redirect below. Any other auth error is a real failure.
  if (authErr && authErr.name !== "AuthSessionMissingError") {
    console.error("[cardgroups] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user) redirect("/login");

  let initialConnection: CardgroupConnection | null = null;
  try {
    const data = await gqlFetch(MyCardgroupsConnectionQuery, {
      variables: { first: CARDGROUPS_PAGE_SIZE },
      revalidate: 0,
    });
    initialConnection = data.myCardgroupsConnection;
  } catch (err) {
    redirectIfUnauthenticated(err, "/login");
  }

  return <CardgroupsClient initialConnection={initialConnection} />;
}
