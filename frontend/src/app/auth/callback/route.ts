import { type NextRequest, NextResponse } from "next/server";
import { createSupabaseServerClient } from "@/lib/supabase/server";

// The post-exchange destination is always `/` — HomePage owns the routing
// decision (see docs/frontend/routing-topology.md). If a return-to is ever
// wired here, route the caller-supplied value through `sanitizeReturnTo`
// (`@/lib/sanitize-return-to`) rather than re-deriving an inline guard.
export async function GET(request: NextRequest) {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");

  if (!code) {
    const loginUrl = new URL("/login", url.origin);
    loginUrl.searchParams.set("error", "missing_code");
    return NextResponse.redirect(loginUrl);
  }

  const supabase = await createSupabaseServerClient();
  const { error } = await supabase.auth.exchangeCodeForSession(code);
  if (error) {
    console.error("[auth/callback] exchangeCodeForSession failed:", error.message);
    const loginUrl = new URL("/login", url.origin);
    loginUrl.searchParams.set("error", "exchange_failed");
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.redirect(new URL("/", url.origin));
}
