import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../../__tests__/utils/mock-supabase";

// next/navigation mock — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const REDIRECT_PREFIX = "REDIRECT:";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`${REDIRECT_PREFIX}${path}`);
  }),
}));

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
}));

// Stub ProfileForm — it is a "use client" component that requires an Apollo
// Provider. The RSC page test only needs to verify props are forwarded
// correctly; the client component has its own dedicated test file.
vi.mock("./profile-form", () => ({
  ProfileForm: ({
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
      ProfileForm
    </div>
  ),
}));

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
    },
  };
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Auth branches
// ---------------------------------------------------------------------------

describe("ProfilePage — auth branches", () => {
  test("anonymous user (user=null, no error) → redirect /login, gqlFetch not called", async () => {
    setMockSupabaseUser(null);

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("AuthSessionMissingError is silenced → redirect /login, no console.error", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    await expect(ProfilePage()).rejects.toThrow(`${REDIRECT_PREFIX}/login`);

    expect(redirect).toHaveBeenCalledWith("/login");
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(gqlFetch).not.toHaveBeenCalled();
  });

  test("non-AuthSessionMissingError → console.error + rethrow", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    await expect(ProfilePage()).rejects.toBe(transportError);

    expect(redirect).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      expect.stringContaining("[profile]"),
      transportError.name,
      transportError.message,
    );
  });
});

// ---------------------------------------------------------------------------
// gqlFetch error branches
// ---------------------------------------------------------------------------

describe("ProfilePage — gqlFetch error branches", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

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
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining("[profile]"), otherErr);
  });
});

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

describe("ProfilePage — happy path", () => {
  beforeEach(() => {
    setMockSupabaseUser({ id: "u-1", email: "user@test.com" });
  });

  test("renders ProfileForm with correct props when user and gqlFetch resolve", async () => {
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

  test("user.email null → ProfileForm receives email=null", async () => {
    // Supabase user without an email address (e.g. OAuth-only account).
    setMockSupabaseUser({ id: "u-1" }); // no email field
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
// JSX tree helpers
// ---------------------------------------------------------------------------

type ProfileFormProps = {
  email: string | null;
  initial: { displayName: string; bio: string };
};

/**
 * Recursively search a React element tree for the ProfileForm stub element and
 * return it so tests can inspect the props forwarded from the page.
 */
function findProfileFormElement(node: unknown): { props: ProfileFormProps } | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;

  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string; displayName?: string }).name === "ProfileForm"
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
