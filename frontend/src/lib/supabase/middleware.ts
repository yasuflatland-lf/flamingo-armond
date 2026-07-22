import { createServerClient } from "@supabase/ssr";
import { type NextRequest, NextResponse } from "next/server";
import { env } from "@/env";
import {
  ONBOARDING_COOKIE_NAME,
  ONBOARDING_COOKIE_TTL_SECONDS,
} from "@/lib/auth/onboarding-cookie";
import {
  ALLOW_ONBOARDING_GATE,
  CLEAR_ONBOARDING_HINT,
  resolveOnboardingGate,
} from "@/lib/auth/onboarding-gate";
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

/**
 * Redirect built off the incoming URL so the origin follows the deployment. The
 * query string is dropped: the target is a self-contained form, and forwarding an
 * arbitrary caller-controlled search onto it only widens the reflected-input surface.
 */
function createRedirectResponse(request: NextRequest, cspPolicy: string | null, target: string) {
  const url = request.nextUrl.clone();
  url.pathname = target;
  url.search = "";
  const response = NextResponse.redirect(url);
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
  let sub = "";
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
      // `?? ""` is not dead: the claim is typed non-null, but a token minted by a
      // non-conforming issuer would leave it undefined and the gate must then
      // decline to bind a cookie rather than bind one to "undefined".
      sub = claimsData.claims.sub ?? "";
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

  // The display-name gate. It runs here, after the identity is verified and before
  // any RSC work, so every route the matcher covers is guarded — including routes
  // added later, which an authenticated-layout gate would silently miss. The
  // per-request cost is a signed-cookie verify; only a cookie miss pays a backend
  // lookup. See docs/frontend/onboarding-gate.md.
  let gate = ALLOW_ONBOARDING_GATE;
  if (authStatus === "authenticated") {
    gate = await resolveOnboardingGate({
      pathname: request.nextUrl.pathname,
      sub,
      cookieValue: request.cookies.get(ONBOARDING_COOKIE_NAME)?.value,
      secret: env.ONBOARDING_GATE_SECRET,
      backendUrl: env.BACKEND_URL,
      getAccessToken: async () => {
        const { data } = await supabase.auth.getSession();
        return data.session?.access_token ?? null;
      },
    });
  } else if (request.cookies.has(ONBOARDING_COOKIE_NAME)) {
    // Sign-out, session expiry, or a cleared browser session: drop the hint so the
    // next signed-in visitor on this browser is re-checked from scratch. An account
    // switch needs no explicit clear — the MAC is bound to the previous `sub` and
    // simply stops verifying.
    gate = CLEAR_ONBOARDING_HINT;
  }

  // Rebuild the forwarded response from the now-complete request headers,
  // preserving any auth cookies the token refresh wrote.
  const finalResponse =
    gate.redirectTo != null
      ? createRedirectResponse(request, cspPolicy, gate.redirectTo)
      : createMiddlewareResponse(requestHeaders, cspPolicy);
  for (const cookie of supabaseResponse.cookies.getAll()) {
    finalResponse.cookies.set(cookie);
  }
  if (gate.setCookie != null) {
    finalResponse.cookies.set(ONBOARDING_COOKIE_NAME, gate.setCookie, {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
      maxAge: ONBOARDING_COOKIE_TTL_SECONDS,
    });
  } else if (gate.clearCookie) {
    finalResponse.cookies.delete(ONBOARDING_COOKIE_NAME);
  }
  return finalResponse;
}
