import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(
    async () => new Headers({ "x-auth-status": "authenticated", "x-user-email": "user@test.com" }),
  ),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Stub ProfilePageClient — it is a "use client" component that requires the
// routing/search-param hooks. The RSC page test only needs to verify props are
// forwarded correctly; the client component has its own dedicated test file.
vi.mock("./profile-page-client", () => ({
  ProfilePageClient: ({
    email,
    initial,
  }: {
    email: string | null;
    initial: { displayName: string; bio: string };
  }) => (
    <div
      data-testid="profile-form"
      data-email={email ?? ""}
      data-display-name={initial.displayName}
    >
      ProfilePageClient
    </div>
  ),
}));

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import ProfilePage from "@/app/profile/page";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeMeData(opts: { displayName?: string; bio?: string } = {}) {
  return {
    me: {
      id: "u-1",
      displayName: opts.displayName ?? "Test User",
      bio: opts.bio ?? "My bio",
      avatarUrl: null,
      learnDisplayMode: "FLIP_TO_REVEAL",
    },
  };
}

function makeRatioData(numerator: number, denominator: number) {
  return { me: { id: "u-1", newCardRatio: { numerator, denominator } } };
}

function setAuthHeaders(status: string, email?: string | null) {
  const h = new Headers({ "x-auth-status": status });
  if (email !== undefined && email !== null) {
    h.set("x-user-email", email);
  }
  vi.mocked(headers).mockResolvedValue(h as Awaited<ReturnType<typeof headers>>);
}

function setAdminAuthHeaders(email = "admin@test.com") {
  vi.mocked(headers).mockResolvedValue(
    new Headers({
      "x-auth-status": "authenticated",
      "x-user-email": email,
      "x-user-is-admin": "true",
    }) as Awaited<ReturnType<typeof headers>>,
  );
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  // Default: authenticated with email
  vi.mocked(headers).mockResolvedValue(
    new Headers({ "x-auth-status": "authenticated", "x-user-email": "user@test.com" }) as Awaited<
      ReturnType<typeof headers>
    >,
  );
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth branches
// ---------------------------------------------------------------------------

describe("ProfilePage — auth branches", () => {
  test("anonymous user (x-auth-status: anonymous) → redirect /login, gqlFetch not called", async () => {
    setAuthHeaders("anonymous");

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("stale session (x-auth-status: stale) → redirect /login, gqlFetch not called", async () => {
    setAuthHeaders("stale");

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("error status (x-auth-status: error) → redirect /login, gqlFetch not called", async () => {
    setAuthHeaders("error");

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("authenticated (x-auth-status: authenticated) → proceeds to gqlFetch", async () => {
    setAuthHeaders("authenticated", "user@test.com");
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeMeData() as never);

    await ProfilePage();

    expect(redirect).not.toHaveBeenCalled();
    expect(gqlFetch).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("ProfilePage — gqlFetch error branches", () => {
  test("UNAUTHENTICATED from gqlFetch → redirect /login (new branch under test)", async () => {
    vi.mocked(gqlFetch).mockRejectedValueOnce(
      new Error(`GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`),
    );

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    // No console.error expected — redirect path swallows the error cleanly.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  test("non-UNAUTHENTICATED gqlFetch error → console.error + rethrow (propagates to error.tsx)", async () => {
    const otherErr = new Error("GraphQL HTTP 500");
    vi.mocked(gqlFetch).mockRejectedValueOnce(otherErr);

    await expect(ProfilePage()).rejects.toBe(otherErr);

    expect(redirect).not.toHaveBeenCalled();
    // PII redaction: only the error name is logged, never the message/object.
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[profile]"), {
      name: "Error",
    });
  });
});

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

describe("ProfilePage — happy path", () => {
  test("renders ProfileForm with correct props when authenticated and gqlFetch resolves", async () => {
    // @vitest-environment jsdom is NOT used here — we avoid rendering to DOM
    // because ProfileForm is stubbed and we only check JSX props directly.
    vi.mocked(gqlFetch).mockResolvedValueOnce(
      makeMeData({ displayName: "Alice", bio: "loves flamingos" }) as never,
    );

    const result = await ProfilePage();

    // The page should not redirect.
    expect(redirect).not.toHaveBeenCalled();

    // Extract props passed to the stubbed ProfileForm by traversing the JSX tree.
    const profileFormEl = findProfileFormElement(result);
    expect(profileFormEl).not.toBeNull();
    expect(profileFormEl?.props.email).toBe("user@test.com");
    expect(profileFormEl?.props.initial).toEqual({
      displayName: "Alice",
      bio: "loves flamingos",
    });
  });

  test("me.displayName null → initial.displayName defaults to empty string", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: null, bio: "some bio", avatarUrl: null },
    } as never);

    const result = await ProfilePage();

    const profileFormEl = findProfileFormElement(result);
    expect(profileFormEl?.props.initial.displayName).toBe("");
  });

  test("me.bio null → initial.bio defaults to empty string", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { id: "u-1", displayName: "Bob", bio: null, avatarUrl: null },
    } as never);

    const result = await ProfilePage();

    const profileFormEl = findProfileFormElement(result);
    expect(profileFormEl?.props.initial.bio).toBe("");
  });

  test("no x-user-email header → ProfileForm receives email=null", async () => {
    // Authenticated user without an email in the forwarded header (e.g. OAuth-only account).
    setAuthHeaders("authenticated");
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeMeData() as never);

    const result = await ProfilePage();

    const profileFormEl = findProfileFormElement(result);
    expect(profileFormEl?.props.email).toBeNull();
  });

  test("me=null → throws (server invariant violated)", async () => {
    vi.mocked(gqlFetch).mockResolvedValueOnce({ me: null } as never);

    await expect(ProfilePage()).rejects.toThrow("/profile: me returned null with no error");

    expect(redirect).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// newCardRatio: admin-only, failure-tolerant secondary fetch
// ---------------------------------------------------------------------------

describe("ProfilePage — newCardRatio (admin-only, failure-tolerant)", () => {
  test("non-admin → newCardRatio is not fetched; only the core query runs", async () => {
    // Default beforeEach headers are authenticated but NOT admin.
    vi.mocked(gqlFetch).mockResolvedValueOnce(makeMeData() as never);

    const result = await ProfilePage();

    // Exactly one gqlFetch: the core Me query. The ratio query is skipped.
    expect(gqlFetch).toHaveBeenCalledTimes(1);
    const el = findProfileFormElement(result);
    expect(el?.props.isAdmin).toBe(false);
  });

  test("admin → newCardRatio fetched via a second query and forwarded", async () => {
    setAdminAuthHeaders();
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(makeMeData() as never) // core Me query
      .mockResolvedValueOnce(makeRatioData(3, 4) as never); // MeNewCardRatio query

    const result = await ProfilePage();

    expect(gqlFetch).toHaveBeenCalledTimes(2);
    const el = findProfileFormElement(result);
    expect(el?.props.isAdmin).toBe(true);
    expect(el?.props.newCardRatio).toEqual({ numerator: 3, denominator: 4 });
  });

  test("admin → newCardRatio fetch failure degrades to the default; profile still renders", async () => {
    setAdminAuthHeaders();
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(makeMeData() as never) // core Me query succeeds
      .mockRejectedValueOnce(
        // Simulates a backend that cannot serve newCardRatio yet (schema desync).
        new Error(
          `GraphQL errors: ${JSON.stringify([
            { message: 'Cannot query field "newCardRatio" on type "User".' },
          ])}`,
        ),
      );

    const result = await ProfilePage();

    // The page does NOT throw and does NOT redirect — the failure is swallowed.
    expect(redirect).not.toHaveBeenCalled();
    const el = findProfileFormElement(result);
    // Falls back to the backend default ratio (4/5).
    expect(el?.props.newCardRatio).toEqual({ numerator: 4, denominator: 5 });
    // The degradation is logged for operator triage, name-only (PII redaction).
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[profile] newCardRatio fetch failed"),
      { name: "Error" },
    );
  });

  test("admin → newCardRatio UNAUTHENTICATED redirects to /login (session expired mid-request)", async () => {
    setAdminAuthHeaders();
    vi.mocked(gqlFetch)
      .mockResolvedValueOnce(makeMeData() as never) // core Me query succeeds
      .mockRejectedValueOnce(
        // A genuine UNAUTHENTICATED (not a schema-desync validation error) still
        // redirects, preserving the repo-wide auth-gated-RSC invariant.
        new Error(
          `GraphQL errors: ${JSON.stringify([{ extensions: { code: "UNAUTHENTICATED" } }])}`,
        ),
      );

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    // The redirect path is taken before the degrade-and-log branch.
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// JSX tree helpers
// ---------------------------------------------------------------------------

type ProfileFormProps = {
  email: string | null;
  initial: { displayName: string; bio: string };
  isAdmin: boolean;
  newCardRatio: { numerator: number; denominator: number };
};

/**
 * Recursively search a React element tree for the ProfilePageClient stub element and
 * return it so tests can inspect the props forwarded from the page.
 */
function findProfileFormElement(node: unknown): { props: ProfileFormProps } | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;

  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string; displayName?: string }).name === "ProfilePageClient"
  ) {
    return { props: el.props as ProfileFormProps };
  }

  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findProfileFormElement(child);
        if (found) return found;
      }
    } else if (children != null) {
      return findProfileFormElement(children);
    }
  }
  return null;
}
