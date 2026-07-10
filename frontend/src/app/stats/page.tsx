import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import type { MyLearningStatsQuery as MyLearningStatsQueryType } from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { MyLearningStatsQuery } from "./queries";
import { StatsClient } from "./stats-client";

export const metadata: Metadata = { title: "Progress" };

export default async function StatsPage() {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect("/login");

  let data: MyLearningStatsQueryType;
  try {
    data = await gqlFetch(MyLearningStatsQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    console.error("[stats] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }
  if (!data.myLearningStats) {
    throw new Error("/stats: myLearningStats returned null with no error");
  }

  return <StatsClient stats={data.myLearningStats} />;
}
