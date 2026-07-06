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

// Core profile every viewer needs. Deliberately excludes newCardRatio: that
// field backs only the admin-only slider, so its availability must never gate
// the core profile load. It is fetched separately in MeNewCardRatioQuery below.
const MeQuery = graphql(`
  query Me {
    me {
      id
      displayName
      bio
      avatarUrl
      learnDisplayMode
    }
  }
`);

// newCardRatio backs the admin-only new-card-ratio slider. It is fetched
// separately, and only for admins, so a backend that cannot yet serve the field
// — e.g. during a rolling deploy where the frontend ships this query before the
// backend serves it — degrades the slider to its default instead of crashing
// the whole /profile page for every user.
const MeNewCardRatioQuery = graphql(`
  query MeNewCardRatio {
    me {
      id
      newCardRatio {
        numerator
        denominator
      }
    }
  }
`);

// Mirrors domain.DefaultNewCardRatio on the backend: 4/5 = 80% new cards. Used
// when the caller is an admin but the ratio fetch fails.
const DEFAULT_NEW_CARD_RATIO = { numerator: 4, denominator: 5 };

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

  // The admin-only ratio is a secondary, optional fetch. A genuine
  // UNAUTHENTICATED (session expired between the two fetches) still redirects to
  // /login, keeping the repo-wide auth-gated-RSC invariant. Any OTHER failure —
  // notably a backend that cannot serve the field yet (schema desync raises a
  // validation error, not UNAUTHENTICATED) — degrades the slider to the default
  // so the core profile above stays up instead of crashing for every user.
  let newCardRatio: { numerator: number; denominator: number } = DEFAULT_NEW_CARD_RATIO;
  if (auth.isAdmin) {
    try {
      const ratioData = await gqlFetch(MeNewCardRatioQuery, { revalidate: 0 });
      if (ratioData.me?.newCardRatio) {
        newCardRatio = ratioData.me.newCardRatio;
      }
    } catch (err) {
      if (isUnauthenticatedGraphQLError(err)) {
        redirect("/login");
      }
      console.error("[profile] newCardRatio fetch failed; using default:", {
        name: err instanceof Error ? err.name : "unknown",
      });
    }
  }

  return (
    <ProfilePageClient
      email={auth.email}
      initial={{
        displayName: data.me.displayName ?? "",
        bio: data.me.bio ?? "",
      }}
      displayMode={data.me.learnDisplayMode}
      newCardRatio={newCardRatio}
      isAdmin={auth.isAdmin}
    />
  );
}
