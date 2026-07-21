import type { Metadata } from "next";
import type { MyLearningStatsQuery as MyLearningStatsQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { MyLearningStatsQuery } from "./queries";
import { StatsClient } from "./stats-client";

export const metadata: Metadata = { title: "Progress" };

export default async function StatsPage() {
  await requireAuthenticated("/login");

  let data: MyLearningStatsQueryType;
  try {
    data = await gqlFetch(MyLearningStatsQuery, { revalidate: 0 });
  } catch (err) {
    redirectIfAuthError(err, "/login");
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
