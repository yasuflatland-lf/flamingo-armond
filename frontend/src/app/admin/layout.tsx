import { headers } from "next/headers";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import { graphql } from "@/generated";
import type { AdminLayoutMeQuery as AdminLayoutMeQueryType } from "@/generated/graphql";
import { gqlFetch } from "@/lib/apollo/server";
import { readAuthContext } from "@/lib/supabase/auth-status";

/**
 * Single source of truth admin gate. Runs entirely on the server before any
 * client component bootstraps, preventing a flash of admin content (FOAC).
 *
 * Order of checks:
 *   1. Middleware-forwarded `x-auth-status` header — `redirect("/")` when the
 *      status is not "authenticated" (anonymous, stale, or error).
 *   2. GraphQL `me` — `redirect("/")` when the resolver returns
 *      UNAUTHENTICATED / FORBIDDEN, or when the returned roles do not include
 *      "admin".
 *
 * Both unauthenticated and non-admin paths redirect to `/` (not `/login`):
 * sending a logged-in non-admin to `/login` is awkward UX, and the home page
 * already routes anonymous visitors to a sign-in CTA.
 *
 * Per-page `x-auth-status` checks under `admin/users/page.tsx` are
 * intentionally retained as defense in depth.
 */
const AdminLayoutMeQuery = graphql(`
  query AdminLayoutMe {
    me {
      id
      roles {
        id
        name
      }
    }
  }
`);

export default async function AdminLayout({ children }: { children: ReactNode }) {
  // Step 1: session check via middleware-forwarded status.
  if (readAuthContext(await headers()).status !== "authenticated") redirect("/");

  // Step 2: admin-role check via GraphQL. UNAUTHENTICATED can still happen
  // here even after Supabase reports a user (e.g., expired access token that
  // the server rejects), so it is folded into the same redirect path.
  let meData: AdminLayoutMeQueryType;
  try {
    meData = await gqlFetch(AdminLayoutMeQuery, { revalidate: 0 });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("UNAUTHENTICATED") || msg.includes("FORBIDDEN")) redirect("/");
    throw err;
  }

  const isAdmin = meData.me?.roles.some((r) => r.name === "admin") ?? false;
  if (!isAdmin) redirect("/");

  // Sub-pages own their <main>; this layout stays a pass-through.
  return <>{children}</>;
}
