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
  const {
    data: { user },
    error,
  } = await supabase.auth.getUser();
  const nonIgnorable = error != null && !isIgnorableAuthError(error);
  if (nonIgnorable) {
    console.error("[supabase/middleware] getUser() failed:", error.name, error.message);
  }

  // isAdmin is a UI hint only (real gate is app/admin/layout.tsx). The admin role
  // claim is injected into the JWT by the Custom Access Token Hook and is NOT
  // mirrored into auth.users app_metadata, so it must be read from the verified
  // claims (getClaims), not from the getUser() user record. getClaims uses
  // jose/WebCrypto and is Edge-compatible.
  let isAdmin = false;
  if (user) {
    const { data: claimsData } = await supabase.auth.getClaims();
    isAdmin = claimsData?.claims?.app_metadata?.role === "admin";
  }
  const authStatus: AuthStatus = user
    ? "authenticated"
    : isStaleSessionError(error)
      ? "stale"
      : nonIgnorable
        ? "error"
        : "anonymous";

  requestHeaders.set(AUTH_STATUS_HEADER, authStatus);
  requestHeaders.set(USER_EMAIL_HEADER, user?.email ?? "");
  requestHeaders.set(USER_IS_ADMIN_HEADER, isAdmin ? "true" : "false");

  // Rebuild the forwarded response from the now-complete request headers,
  // preserving any auth cookies the token refresh wrote.
  const finalResponse = createMiddlewareResponse(requestHeaders, cspPolicy);
  for (const cookie of supabaseResponse.cookies.getAll()) {
    finalResponse.cookies.set(cookie);
  }
  return finalResponse;
}
