import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Table test for the (feAuth, beAuth) redirect chain across HomePage, LoginPage
// and /auth/verify-session: every combination must terminate within MAX_HOPS,
// including a locally-valid token the backend rejects (deleted account).

const state = vi.hoisted(() => ({
  feAuth: "anonymous" as "authenticated" | "anonymous",
  beAuth: "ok" as "ok" | "UNAUTHENTICATED",
  signOutClears: false,
}));

const mockSignOut = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": state.feAuth })),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: vi.fn(async () => ({ auth: { signOut: mockSignOut } })),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(async () => {
    if (state.beAuth === "UNAUTHENTICATED") {
      throw new Error(
        `GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`,
      );
    }
    return {
      me: { id: "u1", displayName: "A", lastViewedCardgroup: null },
      myCardgroupsConnection: { totalCount: 1 },
    };
  }),
}));

vi.mock("next-intl/server", () => ({
  getTranslations: vi.fn(async (namespace: "Login") =>
    createTranslator({ locale: "en", messages: enMessages, namespace }),
  ),
  getLocale: vi.fn(async () => "en"),
}));

// Client components are irrelevant to routing; keep the Supabase browser client out.
vi.mock("@/app/login/login-button", () => ({ LoginButton: () => null }));
vi.mock("@/app/login/in-app-browser-guard", () => ({ InAppBrowserGuard: () => null }));

import { createTranslator } from "next-intl";
import { GET } from "@/app/auth/verify-session/route";
import LoginPage from "@/app/login/page";
import HomePage from "@/app/page";
import { VERIFY_SESSION_PATH } from "@/lib/auth/session-invalid";
import enMessages from "../messages/en.json";

const ORIGIN = "http://localhost";
const MAX_HOPS = 6;
const RENDERED = Symbol("rendered");
const DRIVEN_PATHS = new Set(["/", "/login", VERIFY_SESSION_PATH]);

type Termination = "login-rendered" | "app-surface" | "no-termination";

/** Visits one path and returns the next path, or RENDERED when the page draws. */
async function visit(path: string): Promise<string | typeof RENDERED> {
  const url = new URL(path, ORIGIN);
  try {
    if (url.pathname === "/") {
      await HomePage();
      throw new Error("/ rendered instead of redirecting");
    }
    if (url.pathname === "/login") {
      await LoginPage({ searchParams: Promise.resolve(Object.fromEntries(url.searchParams)) });
      return RENDERED;
    }
    const res = await GET(new NextRequest(`${ORIGIN}${path}`));
    const location = new URL(res.headers.get("location") ?? "", ORIGIN);
    return `${location.pathname}${location.search}`;
  } catch (err) {
    const target = err instanceof Error ? /^REDIRECT:(.*)$/.exec(err.message)?.[1] : undefined;
    if (target !== undefined) return target;
    throw err;
  }
}

async function walk(start: string): Promise<{ route: string[]; end: Termination }> {
  const route = [start];
  let current = start;
  for (let hop = 0; hop < MAX_HOPS; hop++) {
    if (!DRIVEN_PATHS.has(new URL(current, ORIGIN).pathname)) {
      return { route, end: "app-surface" };
    }
    const next = await visit(current);
    if (next === RENDERED) return { route, end: "login-rendered" };
    route.push(next);
    current = next;
  }
  return { route, end: "no-termination" };
}

describe("auth session-invalid redirect chain", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
    mockSignOut.mockImplementation(async () => {
      if (state.signOutClears) state.feAuth = "anonymous";
      return { error: null };
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it.each([
    {
      start: "/",
      feAuth: "anonymous",
      beAuth: "ok",
      signOutClears: false,
      route: ["/", "/login"],
      end: "login-rendered",
      signOutCalls: 0,
    },
    {
      start: "/",
      feAuth: "anonymous",
      beAuth: "UNAUTHENTICATED",
      signOutClears: false,
      route: ["/", "/login"],
      end: "login-rendered",
      signOutCalls: 0,
    },
    {
      start: "/",
      feAuth: "authenticated",
      beAuth: "ok",
      signOutClears: false,
      route: ["/", "/cardgroups"],
      end: "app-surface",
      signOutCalls: 0,
    },
    {
      start: "/",
      feAuth: "authenticated",
      beAuth: "UNAUTHENTICATED",
      signOutClears: true,
      route: ["/", "/login", "/auth/verify-session", "/login?reason=session_invalid"],
      end: "login-rendered",
      signOutCalls: 1,
    },
    {
      start: "/",
      feAuth: "authenticated",
      beAuth: "UNAUTHENTICATED",
      signOutClears: false,
      route: ["/", "/login", "/auth/verify-session", "/login?reason=session_invalid"],
      end: "login-rendered",
      signOutCalls: 1,
    },
    {
      start: "/login",
      feAuth: "authenticated",
      beAuth: "ok",
      signOutClears: false,
      route: ["/login", "/auth/verify-session", "/", "/cardgroups"],
      end: "app-surface",
      signOutCalls: 0,
    },
  ] as const)(
    "start $start, feAuth=$feAuth, beAuth=$beAuth, signOutClears=$signOutClears → $end",
    async ({ start, feAuth, beAuth, signOutClears, route, end, signOutCalls }) => {
      state.feAuth = feAuth;
      state.beAuth = beAuth;
      state.signOutClears = signOutClears;

      const result = await walk(start);

      expect(result.end).toBe(end);
      expect(result.route).toEqual(route);
      expect(mockSignOut).toHaveBeenCalledTimes(signOutCalls);
    },
  );
});
