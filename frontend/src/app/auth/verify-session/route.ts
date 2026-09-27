import { type NextRequest, NextResponse } from "next/server";
import { isUnauthenticatedGraphQLError } from "@/lib/apollo/graphql-errors";
import { gqlFetch } from "@/lib/apollo/server";
import { SESSION_INVALID_REASON } from "@/lib/auth/session-invalid";
import { createSupabaseServerClient } from "@/lib/supabase/server";
import { VerifySessionQuery } from "./queries";

// /login forwards a locally-authenticated visitor here instead of straight to `/`.
// Signing out unconditionally would let any cross-site link log the user out; this
// handler clears only a session the backend already rejects, so `/` <-> `/login`
// cannot loop on a deleted account (see docs/frontend/routing-topology.md).
export async function GET(request: NextRequest) {
  const origin = new URL(request.url).origin;
  try {
    await gqlFetch(VerifySessionQuery, { revalidate: 0 });
  } catch (err) {
    if (!isUnauthenticatedGraphQLError(err)) {
      console.error("[auth/verify-session] gqlFetch failed:", {
        name: err instanceof Error ? err.name : "unknown",
      });
      return NextResponse.redirect(new URL("/", origin));
    }
    const supabase = await createSupabaseServerClient();
    // `local`, not `global`: an aud/iss mismatch rejects a live account, whose
    // other devices must keep their sessions.
    const { error } = await supabase.auth.signOut({ scope: "local" });
    if (error) {
      console.warn("[auth/verify-session] signOut failed:", { name: error.name });
    }
    const loginUrl = new URL("/login", origin);
    loginUrl.searchParams.set("reason", SESSION_INVALID_REASON);
    return NextResponse.redirect(loginUrl);
  }
  return NextResponse.redirect(new URL("/", origin));
}
