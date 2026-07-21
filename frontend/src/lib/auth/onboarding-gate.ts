import { isUserOnboarded } from "@/lib/auth/onboarding";
import {
  ONBOARDING_COOKIE_TTL_SECONDS,
  signOnboardingCookie,
  verifyOnboardingCookie,
} from "@/lib/auth/onboarding-cookie";
import { newRequestId, REQUEST_ID_HEADER } from "@/lib/observability/request-id";

/** Where the gate sends a signed-in caller whose display name is still empty. */
export const ONBOARDING_ENTRY_PATH = "/onboarding";

/**
 * Paths the gate must never redirect, matched on the exact path or a `/`-bounded
 * sub-path. Enumerated deliberately rather than discovered at runtime: an
 * unexempted `/onboarding` is an infinite redirect loop, and an unexempted
 * `/api/healthz` turns a readiness probe into a 307.
 *
 * - `/onboarding` — the redirect target itself, plus `/onboarding/start`.
 * - `/login` — sign-in must stay reachable; it is also where sign-out lands.
 * - `/auth` — the OAuth callback exchanges the code and must complete.
 * - `/api` — route handlers (`/api/ping`, `/api/healthz`) answer machines.
 * - `/_next` — framework assets and RSC payload fetches.
 * - `/terms`, `/privacy` — public legal pages, readable in any account state.
 *
 * The middleware matcher already excludes `/api`, `/auth/callback`, `/_next` and
 * the static files below. The overlap is deliberate: matcher and gate are two
 * independent lists, and the gate must stay correct if the matcher widens.
 */
const GATE_EXEMPT_PREFIXES = [
  "/onboarding",
  "/login",
  "/auth",
  "/api",
  "/_next",
  "/terms",
  "/privacy",
] as const;

/** Static single-file routes served from `public/`, exempt for the same reason. */
const GATE_EXEMPT_PATHS = [
  "/favicon.ico",
  "/sw.js",
  "/offline.html",
  "/manifest.webmanifest",
] as const;

export function isOnboardingGateExempt(pathname: string): boolean {
  if (GATE_EXEMPT_PATHS.some((path) => pathname === path)) return true;
  return GATE_EXEMPT_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

/**
 * Hand-written rather than a codegen `graphql()` document: the middleware runs
 * in the Edge runtime, and printing a TypedDocumentNode would pull graphql-js
 * into that bundle for one three-field query. `onboarding-gate.test.ts` validates
 * this string against `schema/*.graphql`, so it cannot silently drift.
 */
export const ONBOARDING_GATE_QUERY = "query OnboardingGateMe { me { id displayName } }";

/**
 * Authoritative lookup. Returns `null` — deliberately distinct from `false` —
 * when the answer is unknown (transport failure, non-2xx, GraphQL errors), so the
 * caller can fail open instead of stranding every user on `/onboarding` while the
 * backend is down.
 */
async function fetchOnboardedFlag(
  backendUrl: string,
  accessToken: string,
): Promise<boolean | null> {
  let json: { data?: { me?: { displayName?: string | null } | null } | null; errors?: unknown };
  try {
    const res = await fetch(`${backendUrl}/query`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/graphql-response+json, application/json;q=0.9",
        authorization: `Bearer ${accessToken}`,
        [REQUEST_ID_HEADER]: newRequestId(),
      },
      body: JSON.stringify({ query: ONBOARDING_GATE_QUERY }),
      cache: "no-store",
    });
    if (!res.ok) {
      console.warn("[onboarding-gate] display-name lookup returned HTTP", res.status);
      return null;
    }
    json = await res.json();
  } catch (err) {
    // err.message omitted — backend-echoed content can carry user data.
    console.warn(
      "[onboarding-gate] display-name lookup failed:",
      err instanceof Error ? err.name : "unknown",
    );
    return null;
  }
  if (json.errors != null || json.data == null) return null;
  return isUserOnboarded(json.data.me);
}

export type OnboardingGateResult = {
  /** Path to redirect to, or null to let the request through. */
  redirectTo: string | null;
  /** Cookie value the response must (re)issue, or null to leave the cookie alone. */
  setCookie: string | null;
  /** True when the response must delete the hint cookie. */
  clearCookie: boolean;
};

/** No action: let the request through and leave the hint cookie as it is. */
export const ALLOW_ONBOARDING_GATE: OnboardingGateResult = {
  redirectTo: null,
  setCookie: null,
  clearCookie: false,
};

/** Result for a request with no signed-in caller: nothing to gate, drop the hint. */
export const CLEAR_ONBOARDING_HINT: OnboardingGateResult = {
  redirectTo: null,
  setCookie: null,
  clearCookie: true,
};

export type OnboardingGateInput = {
  pathname: string;
  /** `sub` claim of the verified JWT. The cookie's MAC is bound to it. */
  sub: string;
  cookieValue: string | undefined;
  /** Absent when `ONBOARDING_GATE_SECRET` is unset — the fast path is then off. */
  secret: string | undefined;
  backendUrl: string;
  /** Called only on the slow path, so the fast path costs no session read. */
  getAccessToken: () => Promise<string | null>;
  nowMs?: number;
};

let warnedAboutMissingSecret = false;

/**
 * The gate. Exempt paths and a cookie that verifies against this `sub` short-circuit
 * to "allow"; otherwise `isUserOnboarded` is evaluated against a fresh backend
 * lookup, which either issues the fast-path cookie or redirects to onboarding.
 * Every unknown outcome fails open — this guards a UX flow, not authorization.
 */
export async function resolveOnboardingGate(
  input: OnboardingGateInput,
): Promise<OnboardingGateResult> {
  const { pathname, sub, cookieValue, secret, backendUrl, getAccessToken, nowMs } = input;

  if (isOnboardingGateExempt(pathname)) return ALLOW_ONBOARDING_GATE;
  if (sub === "") return ALLOW_ONBOARDING_GATE;

  if (secret === undefined) {
    if (!warnedAboutMissingSecret) {
      warnedAboutMissingSecret = true;
      console.warn(
        `[onboarding-gate] ONBOARDING_GATE_SECRET is unset — the signed fast-path cookie is disabled and every gated navigation costs a backend lookup. Set a secret of at least 32 characters to enable the ${ONBOARDING_COOKIE_TTL_SECONDS}s cache.`,
      );
    }
  } else if (await verifyOnboardingCookie(cookieValue, secret, sub, nowMs)) {
    return ALLOW_ONBOARDING_GATE;
  }

  const accessToken = await getAccessToken();
  if (accessToken == null) return ALLOW_ONBOARDING_GATE;

  const onboarded = await fetchOnboardedFlag(backendUrl, accessToken);
  if (onboarded === null) return ALLOW_ONBOARDING_GATE;
  if (!onboarded) {
    return { redirectTo: ONBOARDING_ENTRY_PATH, setCookie: null, clearCookie: true };
  }
  if (secret === undefined) return ALLOW_ONBOARDING_GATE;
  return {
    redirectTo: null,
    setCookie: await signOnboardingCookie(secret, sub, nowMs),
    clearCookie: false,
  };
}
