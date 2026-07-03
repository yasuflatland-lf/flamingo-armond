import { createServerClient } from "@supabase/ssr";
import { type NextRequest, NextResponse } from "next/server";
import { env } from "@/env";
import { buildHtmlCsp } from "@/lib/security/csp";
import { isIgnorableAuthError, isStaleSessionError } from "@/lib/supabase/auth-errors";
import {
  AUTH_STATUS_HEADER,
  type AuthStatus,
  IDENTITY_HEADERS,
  USER_EMAIL_HEADER,
  USER_IS_ADMIN_HEADER,
} from "@/lib/supabase/auth-status";

const NONCE_BYTES = 18;

function generateNonce(): string {
  const bytes = new Uint8Array(NONCE_BYTES);
  crypto.getRandomValues(bytes);
  return Buffer.from(bytes).toString("base64url");
}

function createMiddlewareResponse(requestHeaders: Headers, cspPolicy: string | null) {
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  if (cspPolicy != null) {
    response.headers.set("Content-Security-Policy", cspPolicy);
  }
  return response;
}

export async function updateSession(request: NextRequest) {
  const requestHeaders = new Headers(request.headers);
  const nonce = generateNonce();

  // Strip any client-supplied identity headers before we compute and set our
  // own — a client must never be able to spoof these. Re-set on every path below.
  for (const name of IDENTITY_HEADERS) {
    requestHeaders.delete(name);
  }

  let cspPolicy: string | null = null;
  try {
    cspPolicy = buildHtmlCsp({
      nonce,
      supabaseUrl: env.NEXT_PUBLIC_SUPABASE_URL,
      allowUnsafeEval: process.env.NODE_ENV !== "production",
    });
  } catch (err) {
    console.error(
      "[supabase/middleware] buildHtmlCsp failed — enforcing Content-Security-Policy and x-nonce omitted from this response:",
      err,
    );
  }

  if (cspPolicy != null) {
    requestHeaders.set("Content-Security-Policy", cspPolicy);
    requestHeaders.set("x-nonce", nonce);
  }

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
          supabaseResponse = createMiddlewareResponse(requestHeaders, cspPolicy);
          for (const { name, value, options } of cookiesToSet) {
            supabaseResponse.cookies.set(name, value, options);
          }
        },
      },
    },
  );

  // getClaims() is the single auth source. It verifies the JWT locally (this
  // project uses asymmetric signing keys and auth-js caches the JWKS in a
  // module-global), and via its internal getSession() call refreshes an expiring
  // token — the setAll cookie hook above captures the rotation. This replaces the
  // former getUser() call, which always sent an Auth-server round trip on every
  // navigation purely to re-verify the same token. getClaims uses jose/WebCrypto
  // and is Edge-compatible.
  //
  // isAdmin is a UI hint only (real gate is app/admin/layout.tsx). The admin role
  // claim is injected into the JWT by the Custom Access Token Hook and is NOT
  // mirrored into auth.users app_metadata, so it is read from the verified claims.
  let authStatus: AuthStatus = "anonymous";
  let email = "";
  let isAdmin = false;
  try {
    // getClaims() has a three-way return: success ({ data: { claims }, error: null }),
    // failure ({ data: null, error }), and anonymous ({ data: null, error: null }).
    const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
    if (claimsError != null) {
      // A refresh/verification failure. Deleted-user detection now lands here (at
      // the token-refresh boundary) instead of on getUser(); it is bounded by the
      // jwt_expiry refresh cadence (supabase/config.toml).
      if (isStaleSessionError(claimsError)) {
        authStatus = "stale";
      } else if (isIgnorableAuthError(claimsError)) {
        authStatus = "anonymous";
      } else {
        authStatus = "error";
        // err.message omitted — a Supabase auth error message may carry user-identifying content.
        console.error("[supabase/middleware] getClaims() failed:", claimsError.name);
      }
    } else if (claimsData == null) {
      // Anonymous request: no session. getClaims() returns { data: null, error: null }
      // (NOT AuthSessionMissingError) in this case.
      authStatus = "anonymous";
    } else {
      authStatus = "authenticated";
      email = claimsData.claims.email ?? "";
      isAdmin = claimsData.claims.app_metadata?.role === "admin";
    }
  } catch (err) {
    // getClaims() can throw non-AuthError exceptions (plain Error from validateExp,
    // DOMException from WebCrypto) that escape the SDK's internal AuthError catch.
    // Fail closed to the degraded (logo-only) shell — an uncaught throw would 500
    // the whole request.
    authStatus = "error";
    console.warn(
      "[supabase/middleware] getClaims() threw unexpectedly — degrading to error status:",
      err instanceof Error ? err.name : "unknown",
    );
  }

  requestHeaders.set(AUTH_STATUS_HEADER, authStatus);
  requestHeaders.set(USER_EMAIL_HEADER, email);
  requestHeaders.set(USER_IS_ADMIN_HEADER, isAdmin ? "true" : "false");

  // Rebuild the forwarded response from the now-complete request headers,
  // preserving any auth cookies the token refresh wrote.
  const finalResponse = createMiddlewareResponse(requestHeaders, cspPolicy);
  for (const cookie of supabaseResponse.cookies.getAll()) {
    finalResponse.cookies.set(cookie);
  }
  return finalResponse;
}
