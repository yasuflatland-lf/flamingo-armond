import type { Metadata } from "next";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import type {
  LegacyMyLearningStatsQuery as LegacyMyLearningStatsQueryType,
  MyLearningStatsQuery as MyLearningStatsQueryType,
} from "@/generated/graphql";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";
import { LegacyMyLearningStatsQuery, MyLearningStatsQuery } from "./queries";
import { StatsClient } from "./stats-client";

export const metadata: Metadata = { title: "Progress" };

function isPerformanceWindowsUnavailable(err: unknown): boolean {
  return (
    err instanceof Error &&
    err.message.includes("Cannot query field") &&
    err.message.includes("performanceWindows") &&
    err.message.includes("LearningStats")
  );
}

function adaptLegacyStats(data: LegacyMyLearningStatsQueryType): MyLearningStatsQueryType {
  const { performance, ...stats } = data.myLearningStats;
  return {
    myLearningStats: {
      ...stats,
      performanceWindows: {
        __typename: "PerformanceWindows",
        // The old API exposes only the 365-day snapshot. Reusing it keeps the
        // page functional during rollout; real shorter windows arrive as soon
        // as the additive backend schema is live.
        days365: performance,
        days30: performance,
        days7: performance,
      },
    },
  };
}

export default async function StatsPage() {
  const auth = readAuthContext(await headers());
  if (auth.status !== "authenticated") redirect("/login");

  let data: MyLearningStatsQueryType;
  let performanceWindowsAvailable = true;
  try {
    data = await gqlFetch(MyLearningStatsQuery, { revalidate: 0 });
  } catch (err) {
    if (isUnauthenticatedGraphQLError(err)) redirect("/login");
    if (isPerformanceWindowsUnavailable(err)) {
      try {
        const legacyData = await gqlFetch(LegacyMyLearningStatsQuery, { revalidate: 0 });
        data = adaptLegacyStats(legacyData);
        performanceWindowsAvailable = false;
      } catch (legacyErr) {
        if (isUnauthenticatedGraphQLError(legacyErr)) redirect("/login");
        console.error("[stats] legacy gqlFetch failed:", {
          name: legacyErr instanceof Error ? legacyErr.name : "unknown",
        });
        throw legacyErr;
      }
    } else {
      console.error("[stats] gqlFetch failed:", {
        name: err instanceof Error ? err.name : "unknown",
      });
      throw err;
    }
  }
  if (!data.myLearningStats) {
    throw new Error("/stats: myLearningStats returned null with no error");
  }

  return (
    <StatsClient
      stats={data.myLearningStats}
      performanceWindowsAvailable={performanceWindowsAvailable}
    />
  );
}
