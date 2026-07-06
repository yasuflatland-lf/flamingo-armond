import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { graphql } from "@/generated";
import type { MeQuery as MeQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { ProfilePageClient } from "./profile-page-client";

export const metadata: Metadata = { title: "Profile" };

const MeQuery = graphql(`
  query Me {
    me {
      id
      displayName
      bio
      avatarUrl
      learnDisplayMode
      newCardRatio {
        numerator
        denominator
      }
    }
  }
`);

export default async function ProfilePage() {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect("/login");

  let data: MeQueryType;
  try {
    data = await gqlFetch(MeQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) {
      redirect("/login");
    }
    console.error("[profile] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }
  if (!data.me) {
    throw new Error("/profile: me returned null with no error");
  }

  return (
    <ProfilePageClient
      email={auth.email}
      initial={{
        displayName: data.me.displayName ?? "",
        bio: data.me.bio ?? "",
      }}
      displayMode={data.me.learnDisplayMode}
      newCardRatio={data.me.newCardRatio}
      isAdmin={auth.isAdmin}
    />
  );
}
