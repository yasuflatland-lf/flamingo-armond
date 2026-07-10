"use client";

import { useLazyQuery } from "@apollo/client/react";
import { useCallback } from "react";
import { MergeMasterCardgroupPreviewQuery } from "@/app/catalog/queries";
import { classifyAndLogAuthOutcome } from "@/lib/apollo/errors";

export type MergeFromCatalogPreviewOutcome =
  | { status: "success"; addedCount: number; updatedCount: number }
  | { status: "not_found" }
  | { status: "auth"; kind: "unauthenticated" | "forbidden" }
  | { status: "rejected" };

/**
 * Dry-run preview of `mergeMasterCardgroup`. Returns the projected add/update tally
 * for merging `masterCardgroupId` into `targetCardgroupId`, without writing.
 * Mirrors the outcome union of useMergeFromCatalog so the sheet can branch uniformly.
 *
 * Apollo v4 `useLazyQuery` resolves (never rejects) with `result.error` set on
 * transport failure. Both the `result.error` branch and the `try/catch` are active:
 * the former covers the resolved-with-error path; the latter covers unexpected throws.
 */
export function useMergeFromCatalogPreview(targetCardgroupId: string) {
  const [runPreview, { loading }] = useLazyQuery(MergeMasterCardgroupPreviewQuery, {
    fetchPolicy: "network-only",
  });

  const previewMerge = useCallback(
    async (masterCardgroupId: string): Promise<MergeFromCatalogPreviewOutcome> => {
      try {
        const result = await runPreview({
          variables: { input: { masterCardgroupId, cardgroupId: targetCardgroupId } },
        });
        const payload = result.data?.mergeMasterCardgroupPreview;
        const typename = payload?.__typename ?? null;
        if (payload?.__typename === "MasterNotFoundError") {
          return { status: "not_found" };
        }
        if (payload?.__typename === "MergeMasterCardgroupPreview") {
          return {
            status: "success",
            addedCount: payload.addedCount,
            updatedCount: payload.updatedCount,
          };
        }
        if (result.error) {
          return classifyAndLogAuthOutcome(
            result.error,
            "useMergeFromCatalogPreview",
            "mergeMasterCardgroupPreview",
            { targetCardgroupId },
          );
        }
        // Null payload, partial-response null bubble, or a future union variant.
        console.warn("[useMergeFromCatalogPreview] unexpected preview payload", {
          typename,
          targetCardgroupId,
        });
        return { status: "rejected" };
      } catch (err) {
        return classifyAndLogAuthOutcome(
          err,
          "useMergeFromCatalogPreview",
          "mergeMasterCardgroupPreview",
          { targetCardgroupId },
        );
      }
    },
    [runPreview, targetCardgroupId],
  );

  return { previewMerge, loading };
}
