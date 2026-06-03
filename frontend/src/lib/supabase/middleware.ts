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
    // err.message omitted — a Supabase auth error message may carry user-identifying content.
    console.error("[supabase/middleware] getUser() failed:", error.name);
  }

  // isAdmin is a UI hint only (real gate is app/admin/layout.tsx). The admin role
  // claim is injected into the JWT by the Custom Access Token Hook and is NOT
  // mirrored into auth.users app_metadata, so it must be read from the verified
  // claims (getClaims), not from the getUser() user record. getClaims uses
  // jose/WebCrypto and is Edge-compatible.
  let isAdmin = false;
  if (user) {
    try {
      const { data: claimsData, error: claimsError } = await supabase.auth.getClaims();
      if (claimsError) {
        // err.message omitted — a Supabase auth error message may carry user-identifying content.
        console.warn(
          "[supabase/middleware] getClaims() failed — isAdmin defaulting to false:",
          claimsError.name,
        );
      }
      isAdmin = claimsData?.claims?.app_metadata?.role === "admin";
    } catch (err) {
      // getClaims() can throw non-AuthError exceptions (plain Error from validateExp,
      // DOMException from WebCrypto) that escape the SDK's internal AuthError catch.
      // isAdmin is a UI hint only (real gate is app/admin/layout.tsx), so fail closed.
      console.warn(
        "[supabase/middleware] getClaims() threw unexpectedly — isAdmin defaulting to false:",
        err instanceof Error ? err.name : "unknown",
      );
    }
  }
  let authStatus: AuthStatus;
  if (user) {
    authStatus = "authenticated";
  } else if (isStaleSessionError(error)) {
    authStatus = "stale";
  } else if (nonIgnorable) {
    authStatus = "error";
  } else {
    authStatus = "anonymous";
  }

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
