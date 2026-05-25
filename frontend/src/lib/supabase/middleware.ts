import { createServerClient } from "@supabase/ssr";
import { type NextRequest, NextResponse } from "next/server";
import { env } from "@/env";
import { buildHtmlReportOnlyCsp } from "@/lib/security/csp";
import { isIgnorableAuthError } from "@/lib/supabase/auth-errors";

const NONCE_BYTES = 18;
const BASE64URL_CHARS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

function generateNonce() {
  const bytes = new Uint8Array(NONCE_BYTES);
  crypto.getRandomValues(bytes);

  let nonce = "";
  for (let index = 0; index < bytes.length; index += 3) {
    const chunk =
      ((bytes[index] ?? 0) << 16) | ((bytes[index + 1] ?? 0) << 8) | (bytes[index + 2] ?? 0);
    nonce += BASE64URL_CHARS.charAt((chunk >> 18) & 63);
    nonce += BASE64URL_CHARS.charAt((chunk >> 12) & 63);
    nonce += BASE64URL_CHARS.charAt((chunk >> 6) & 63);
    nonce += BASE64URL_CHARS.charAt(chunk & 63);
  }

  return nonce;
}

function createMiddlewareResponse(requestHeaders: Headers, reportOnlyPolicy: string | null) {
  const response = NextResponse.next({
    request: { headers: requestHeaders },
  });
  if (reportOnlyPolicy != null) {
    response.headers.set("Content-Security-Policy-Report-Only", reportOnlyPolicy);
  }
  return response;
}

export async function updateSession(request: NextRequest) {
  // Forward the request pathname as a header so server components can read it
  // via `next/headers` (no usePathname in RSC). AppShell uses this to skip the
  // navigation rail on /login.
  const requestHeaders = new Headers(request.headers);
  const nonce = generateNonce();

  let reportOnlyPolicy: string | null = null;
  try {
    reportOnlyPolicy = buildHtmlReportOnlyCsp({
      nonce,
      supabaseUrl: env.NEXT_PUBLIC_SUPABASE_URL,
    });
  } catch (err) {
    console.error(
      "[supabase/middleware] buildHtmlReportOnlyCsp failed, CSP header will be omitted:",
      err instanceof Error ? err.message : String(err),
    );
  }

  if (reportOnlyPolicy != null) {
    // Named "Content-Security-Policy" so Next's SSR pipeline can extract the
    // nonce for <script nonce="..."> injection — this is an internal forwarding
    // header, not an enforcement policy. The browser-visible header is set on
    // the response as Content-Security-Policy-Report-Only below.
    requestHeaders.set("Content-Security-Policy", reportOnlyPolicy);
    requestHeaders.set("x-nonce", nonce);
  }
  requestHeaders.set("x-pathname", request.nextUrl.pathname);

  let supabaseResponse = createMiddlewareResponse(requestHeaders, reportOnlyPolicy);

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
          supabaseResponse = createMiddlewareResponse(requestHeaders, reportOnlyPolicy);
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
