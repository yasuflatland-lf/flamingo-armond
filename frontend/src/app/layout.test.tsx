import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseClaims,
  setMockSupabaseClaimsError,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../__tests__/utils/mock-supabase";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any import of the module under test.
// ---------------------------------------------------------------------------

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

// AppShell, Providers, and GlobalFAB are opaque to this test — we do not need
// to render them; we only need to inspect the props the layout passes to AppShell.
// Stubs are defined as minimal functions so React's JSX type system is satisfied.
vi.mock("@/components/nav/app-shell", () => ({
  AppShell: (props: unknown) => props,
}));

vi.mock("@/app/providers", () => ({
  Providers: ({ children }: { children?: React.ReactNode }) => children,
}));

vi.mock("@/components/nav/global-fab", () => ({
  GlobalFAB: () => null,
}));

// next/navigation — the root layout does not redirect, but transitive imports
// may reference navigation hooks; provide a minimal stub.
vi.mock("next/navigation", () => ({
  redirect: vi.fn((path: string) => {
    throw new Error(`REDIRECT:${path}`);
  }),
  usePathname: vi.fn(() => "/"),
}));

// next/headers — middleware forwards the request pathname via x-pathname so
// the server component can decide whether to mount AppShell. Tests override
// the return value for the /login bypass case.
const mockGetHeader = vi.fn<(name: string) => string | null>(() => null);
vi.mock("next/headers", () => ({
  headers: vi.fn(() => Promise.resolve({ get: mockGetHeader })),
}));

// ---------------------------------------------------------------------------
// Import after mocks are registered.
// ---------------------------------------------------------------------------

import RootLayout from "@/app/layout";

// ---------------------------------------------------------------------------
// Helper: traverse the React element tree returned by RootLayout and find
// the props passed to AppShell.
// The layout returns: <html><body><Providers><AppShell ...>...</AppShell></Providers></body></html>
// AppShell receives `user` and `isAdmin` — those are the values under test.
// ---------------------------------------------------------------------------

type AppShellProps = {
  user: { email: string | null } | null;
  isAdmin: boolean;
};

/**
 * Recursively walk a React element tree and return the props of the first
 * element whose `type` function is named `componentName`.
 */
function findElementProps(node: unknown, componentName: string): AppShellProps | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as Record<string, unknown>;
  if (
    "type" in el &&
    "props" in el &&
    typeof el.type === "function" &&
    (el.type as { name?: string; displayName?: string }).name === componentName
  ) {
    return el.props as AppShellProps;
  }
  if ("props" in el && el.props != null) {
    const children = (el.props as Record<string, unknown>).children;
    if (Array.isArray(children)) {
      for (const child of children) {
        const found = findElementProps(child, componentName);
        if (found != null) return found;
      }
    } else if (children != null) {
      return findElementProps(children, componentName);
    }
  }
  return null;
}

/**
 * Invoke RootLayout and locate the props forwarded to AppShell. Throws if
 * AppShell is not found in the returned JSX tree (which would indicate the
 * layout structure changed in a way that breaks the test assumptions).
 */
async function renderLayoutAndGetShellProps(
  children: React.ReactNode = <div />,
): Promise<AppShellProps> {
  const tree = await RootLayout({ children });
  const props = findElementProps(tree, "AppShell");
  if (props == null) {
    throw new Error(
      "AppShell element not found in the RootLayout JSX tree — the layout structure may have changed.",
    );
  }
  return props;
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.clearAllMocks();
  resetMockSupabase();
  // Default: x-pathname is absent, so layout falls through to the AppShell
  // branch. The /login test overrides this per-case.
  mockGetHeader.mockReturnValue(null);
  consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
});

afterEach(() => {
  // Restore in LIFO order so each spy is unwound in the reverse of installation.
  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();
});

// ---------------------------------------------------------------------------
// Test suite: root layout degradation branches
// ---------------------------------------------------------------------------

describe("RootLayout — error handling and AppShell prop wiring", () => {
  // Case 1: AuthSessionMissingError is the normal anonymous-request signal.
  // The layout must NOT call getClaims, must NOT emit console.error, and must
  // pass user=null / isAdmin=false to AppShell.
  test("AuthSessionMissingError: no getClaims call, no console output, AppShell gets user=null isAdmin=false", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case 2: A non-AuthSessionMissingError from getUser() (e.g. a transport
  // failure) must be logged with the [layout] prefix and degrade gracefully —
  // AppShell receives a null user and false isAdmin, no getClaims is called.
  test("non-AuthSessionMissingError from getUser: console.error with [layout] prefix, degraded shell, no getClaims", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[layout] getUser() failed:",
      transportError.name,
      transportError.message,
    );
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case 3: Authenticated user whose claims contain app_metadata.role === "admin".
  // AppShell must receive user.email and isAdmin=true.
  test("authenticated user + admin role in claims: AppShell gets user.email and isAdmin=true", async () => {
    setMockSupabaseUser({ id: "u-3", email: "admin@example.com" });
    setMockSupabaseClaims({ app_metadata: { role: "admin" } });

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(props.user).toEqual({ email: "admin@example.com" });
    expect(props.isAdmin).toBe(true);
  });

  // Case 4: Authenticated user whose claims contain a non-admin role.
  test("authenticated user + non-admin role in claims: isAdmin=false", async () => {
    setMockSupabaseUser({ id: "u-4", email: "member@example.com" });
    setMockSupabaseClaims({ app_metadata: { role: "member" } });

    const props = await renderLayoutAndGetShellProps();

    expect(props.isAdmin).toBe(false);
    expect(props.user).toEqual({ email: "member@example.com" });
  });

  // Case 5: Authenticated user whose claims have app_metadata.role absent.
  test("authenticated user + claims with role missing: isAdmin=false", async () => {
    setMockSupabaseUser({ id: "u-5", email: "new@example.com" });
    setMockSupabaseClaims({ app_metadata: {} });

    const props = await renderLayoutAndGetShellProps();

    expect(props.isAdmin).toBe(false);
  });

  // Case 6: Authenticated user whose claims have app_metadata absent entirely.
  test("authenticated user + claims with app_metadata absent: isAdmin=false", async () => {
    setMockSupabaseUser({ id: "u-6a", email: "bare@example.com" });
    setMockSupabaseClaims({});

    const props = await renderLayoutAndGetShellProps();

    expect(props.isAdmin).toBe(false);
  });

  // Case 7: Authenticated user but getClaims returns an error.
  // The layout must:
  //   - emit console.warn with "[layout] getClaims() failed"
  //   - include a structured payload with a discriminating key (user_id) so
  //     the assertion cannot be satisfied by a bare Error instance — per
  //     docs/frontend/typescript-conventions.md "expect.objectContaining
  //     ({ message }) is not enough"
  //   - NOT include email or display_name in the payload (PII protection) — per
  //     docs/backend/error-wrapping/assert-pii-absence-on-log-lines.md
  //   - degrade to isAdmin=false without throwing
  //   - still pass user.email to AppShell (only the admin flag is degraded)
  test("authenticated user + getClaims error: console.warn with discriminating payload, PII absent, isAdmin=false", async () => {
    const userId = "u-7";
    setMockSupabaseUser({ id: userId, email: "private@example.com" });
    const claimsErr = new Error("JWKS fetch failed");
    claimsErr.name = "JWKSFetchError";
    setMockSupabaseClaimsError(claimsErr);

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).not.toHaveBeenCalled();

    // The warn must be called with the exact [layout] scope prefix.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[layout] getClaims() failed",
      // `user_id` is a discriminating own-property key that Error.prototype
      // does not carry — it distinguishes the intended structured log object
      // from a plain Error regression.
      expect.objectContaining({
        user_id: userId,
      }),
    );

    // PII absence: email and display_name must NOT appear in the warn payload.
    const warnPayload = consoleWarnSpy.mock.calls[0][1] as Record<string, unknown>;
    expect(Object.keys(warnPayload)).not.toContain("email");
    expect(Object.keys(warnPayload)).not.toContain("display_name");

    expect(props.isAdmin).toBe(false);
    // user.email is still wired to AppShell — the shell is not degraded when
    // only the claims lookup fails.
    expect(props.user).toEqual({ email: "private@example.com" });
  });

  // Case 8: Anonymous user — user is null with no error (e.g. a clean
  // signed-out state). The layout must short-circuit getClaims and pass
  // null / false to AppShell.
  test("anonymous user (user=null, no error): no getClaims call, AppShell gets user=null isAdmin=false", async () => {
    setMockSupabaseUser(null); // explicit for readability; also the default after reset

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case 9: /login route bypasses AppShell entirely. The layout must NOT call
  // Supabase getUser() (the auth page handles its own session check) and the
  // returned tree must NOT contain an AppShell element. The supabase server
  // client mock would still be invocable, but the layout must short-circuit
  // before reaching it.
  test("/login: layout renders bare children without AppShell, no Supabase calls", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-pathname" ? "/login" : null));

    const tree = await RootLayout({ children: <div data-testid="login-children" /> });
    const appShellProps = findElementProps(tree, "AppShell");

    expect(appShellProps).toBeNull();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
  });
});
