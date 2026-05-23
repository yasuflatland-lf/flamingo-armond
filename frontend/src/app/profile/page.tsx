import { redirect } from "next/navigation";
import { graphql } from "@/generated";
import type { MeQuery as MeQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { ProfilePageClient } from "./profile-page-client";

const MeQuery = graphql(`
  query Me {
    me {
      id
      displayName
      bio
      avatarUrl
    }
  }
`);

export default async function ProfilePage() {
  const supabase = await createSupabaseServerClient();
  const {
    data: { user },
    error: authErr,
  } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request; stale session = deleted user
  // with a still-valid JWT. Both are handled by redirecting to /login.
  if (authErr && !isIgnorableAuthError(authErr)) {
    console.error("[profile] getUser() failed:", authErr.name, authErr.message);
    throw authErr;
  }
  if (!user || isStaleSessionError(authErr)) redirect("/login");

  let data: MeQueryType;
  try {
    data = await gqlFetch(MeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[profile] gqlFetch failed:", err);
    throw err;
  }
  if (!data.me) {
    throw new Error("/profile: me returned null with no error");
  }

  return (
    <ProfilePageClient
      email={user.email ?? null}
      initial={{
        displayName: data.me.displayName ?? "",
        bio: data.me.bio ?? "",
      }}
    />
  );
}
