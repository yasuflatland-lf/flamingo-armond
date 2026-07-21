import type { Metadata } from "next";
import { Suspense } from "react";
import type { MasterCatalogQuery as MasterCatalogQueryType } from "@/generated/graphql";
import { redirectIfAuthError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { requireAuthenticated } from "@/lib/supabase/auth-status";
import { CatalogSkeleton } from "./_components/catalog-skeleton";
import CatalogClient from "./catalog-client";
import { CATALOG_DEFAULT_VARS, MasterCatalogQuery } from "./queries";

export const metadata: Metadata = { title: "Catalog" };

export default async function CatalogPage() {
  // Auth check runs OUTSIDE the Suspense boundary so an unauthenticated request
  // redirects to /login before any streaming starts. If the redirect ran from
  // within the suspended subtree, the skeleton would flash before the navigation.
  await requireAuthenticated("/login");

  return (
    <Suspense fallback={<CatalogSkeleton />}>
      <CatalogContent />
    </Suspense>
  );
}

/**
 * Inner async server component that performs the GraphQL fetch. Extracted from
 * `CatalogPage` so the route shell can stream while this query resolves.
 *
 * Exported as a named export (not the default) so the RSC test can invoke it
 * directly without going through React's Suspense renderer; the outer page test
 * verifies the Suspense boundary and skeleton fallback separately.
 */
export async function CatalogContent() {
  let data: MasterCatalogQueryType;
  try {
    data = await gqlFetch(MasterCatalogQuery, {
      variables: CATALOG_DEFAULT_VARS,
      revalidate: 0,
    });
  } catch (err) {
    // Structural parse per .claude/rules/frontend-rsc-error-handling.md §
    // "Structurally parse GraphQL extensions.code — never substring-match".
    redirectIfAuthError(err, "/login");
    console.error("[catalog] gqlFetch failed:", {
      name: err instanceof Error ? err.name : "unknown",
    });
    throw err;
  }

  const initialConnection = data.masterCatalog;
  if (!initialConnection) {
    console.error("[catalog] masterCatalog is null — partial response from backend");
    throw new Error("masterCatalog missing from catalog data");
  }

  return <CatalogClient initialConnection={initialConnection} />;
}
