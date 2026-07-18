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

// Detect an old backend that predates the `performanceWindows` field by
// matching the GraphQL validation error text. This is a DELIBERATE, SCOPED
// exception to this repo's convention of parsing `extensions.code` rather than
// substring-matching a GraphQL error message: validation-phase errors
// ("Cannot query field ...") carry only the generic `GRAPHQL_VALIDATION_FAILED`
// code with no field identity, so — unlike the `UNAUTHENTICATED` / `FORBIDDEN`
// codes parsed structurally elsewhere in this file — the offending field name
// is recoverable only from the message text. This is a temporary rollout shim,
// removable once every backend exposes `performanceWindows`.
function isPerformanceWindowsUnavailable(err: unknown): boolean {
  return (
    err instanceof Error &&
    err.message.includes("Cannot query field") &&
    err.message.includes("performanceWindows") &&
    err.message.includes("LearningStats")
  );
}

function adaptLegacyStats(data: LegacyMyLearningStatsQueryType): MyLearningStatsQueryType {
  // Mirror the primary-path null-guard (see StatsPage below): a partial GraphQL
  // response can carry `myLearningStats: null` alongside a non-auth error, in
  // which case gqlFetch returns the data instead of throwing. Surface that as a
  // clear invariant error rather than letting the destructure throw a raw
  // TypeError that the fetch catch would then mislabel as a "gqlFetch failed".
  if (!data.myLearningStats) {
    throw new Error("/stats: legacy myLearningStats returned null with no error");
  }
  const { performance, ...stats } = data.myLearningStats;
  if (!performance) {
    throw new Error("/stats: legacy performance returned null with no error");
  }
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
      let legacyData: LegacyMyLearningStatsQueryType;
      try {
        legacyData = await gqlFetch(LegacyMyLearningStatsQuery, { revalidate: 0 });
      } catch (legacyErr) {
        if (isUnauthenticatedGraphQLError(legacyErr)) redirect("/login");
        console.error("[stats] legacy gqlFetch failed:", {
          name: legacyErr instanceof Error ? legacyErr.name : "unknown",
        });
        throw legacyErr;
      }
      // adaptLegacyStats guards the response shape and throws a clear invariant
      // error on null data. Keep it outside the fetch try/catch so a shape
      // failure surfaces on its own, not mislabeled as a "gqlFetch failed".
      data = adaptLegacyStats(legacyData);
      performanceWindowsAvailable = false;
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
