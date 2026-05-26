import { createServerClient } from "@supabase/ssr";
import { type NextRequest, NextResponse } from "next/server";
import { env } from "@/env";
import { buildHtmlCsp } from "@/lib/security/csp";
import { isIgnorableAuthError } from "@/lib/supabase/auth-errors";

const NONCE_BYTES = 18;

function generateNonce(): string {
  const bytes = new Uint8Array(NONCE_BYTES);
  crypto.getRandomValues(bytes);
  return Buffer.from(bytes).toString("base64url");
}

function createMiddlewareResponse(requestHeaders: Headers, cspPolicy: string | null) {
  const response = NextResponse.next({
    request: { headers: requestHeaders },
  });
  if (cspPolicy != null) {
    response.headers.set("Content-Security-Policy", cspPolicy);
  }
  return response;
}

export async function updateSession(request: NextRequest) {
  // Forward the request pathname as a header so server components can read it
  // via `next/headers` (no usePathname in RSC). AppShell uses this to skip the
  // navigation rail on /login.
  const requestHeaders = new Headers(request.headers);
  const nonce = generateNonce();

  let cspPolicy: string | null = null;
  try {
    cspPolicy = buildHtmlCsp({
      nonce,
      supabaseUrl: env.NEXT_PUBLIC_SUPABASE_URL,
    });
  } catch (err) {
    console.error(
      "[supabase/middleware] buildHtmlCsp failed — enforcing Content-Security-Policy and x-nonce omitted from this response:",
      err,
    );
  }

  if (cspPolicy != null) {
    // Forward the policy as a request header so Next's SSR pipeline can extract
    // the nonce for <script nonce="..."> injection. This is an internal header
    // consumed by the rendering pipeline; the browser never sees it. The
    // enforcing response header is set in createMiddlewareResponse() below.
    // When cspPolicy is null (buildHtmlCsp threw), both headers are omitted and
    // auth/routing/cookie refresh continue normally.
    requestHeaders.set("Content-Security-Policy", cspPolicy);
    requestHeaders.set("x-nonce", nonce);
  }
  requestHeaders.set("x-pathname", request.nextUrl.pathname);

  let supabaseResponse = createMiddlewareResponse(requestHeaders, cspPolicy);

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
          // request headers so x-pathname, nonce, and CSP survive.
          supabaseResponse = createMiddlewareResponse(requestHeaders, cspPolicy);
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
