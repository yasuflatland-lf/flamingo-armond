import { createServerClient } from "@supabase/ssr";
import { type NextRequest, NextResponse } from "next/server";
import { env } from "@/env";
import { isIgnorableAuthError } from "@/lib/supabase/auth-errors";

export async function updateSession(request: NextRequest) {
  // Forward the request pathname as a header so server components can read it
  // via `next/headers` (no usePathname in RSC). AppShell uses this to skip the
  // navigation rail on /login.
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-pathname", request.nextUrl.pathname);

  let supabaseResponse = NextResponse.next({
    request: { headers: requestHeaders },
  });

  const supabase = createServerClient(
    env.NEXT_PUBLIC_SUPABASE_URL,
    env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
    {
      cookies: {
        getAll() {
          return request.cookies.getAll();
        },
        setAll(cookiesToSet) {
          for (const { name, value } of cookiesToSet) {
            request.cookies.set(name, value);
          }
          // Re-create the response after cookie writes; pass the same modified
          // request headers so x-pathname survives.
          supabaseResponse = NextResponse.next({
            request: { headers: requestHeaders },
          });
          for (const { name, value, options } of cookiesToSet) {
            supabaseResponse.cookies.set(name, value, options);
          }
        },
      },
    },
  );

  // CRITICAL: getUser() is what triggers token refresh — removing this call
  // silently breaks session renewal, leaving users with expired tokens.
  const { error } = await supabase.auth.getUser();
  // AuthSessionMissingError = anonymous request (not actionable — would flood
  // edge-runtime stderr). Stale session = deleted user with a still-valid JWT
  // (RSC pages redirect to /login; no log needed at the middleware layer).
  if (error && !isIgnorableAuthError(error)) {
    console.error("[supabase/middleware] getUser() failed:", error.name, error.message);
  }

  return supabaseResponse;
}
