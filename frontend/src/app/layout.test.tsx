import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  mockSupabaseServerClient,
  resetMockSupabase,
  setMockSupabaseUser,
  setMockSupabaseUserError,
} from "../../__tests__/utils/mock-supabase";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any import of the module under test.
// ---------------------------------------------------------------------------

vi.mock("@/lib/supabase/server", () => ({
  createSupabaseServerClient: () => Promise.resolve(mockSupabaseServerClient()),
}));

vi.mock("@/lib/apollo/server", () => ({
  gqlFetch: vi.fn(),
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

// ---------------------------------------------------------------------------
// Import after mocks are registered.
// ---------------------------------------------------------------------------

import RootLayout from "@/app/layout";
import { gqlFetch } from "@/lib/apollo/server";

// ---------------------------------------------------------------------------
// Helper: build the UNAUTHENTICATED error exactly as gqlFetch throws it.
// The `isUnauthenticatedGraphQLError` helper checks for the prefix
// "GraphQL errors: " followed by a JSON array with at least one entry whose
// extensions.code equals "UNAUTHENTICATED".
// ---------------------------------------------------------------------------

function makeUnauthenticatedError(): Error {
  const errors = [{ extensions: { code: "UNAUTHENTICATED" } }];
  return new Error(`GraphQL errors: ${JSON.stringify(errors)}`);
}

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
  // The layout must NOT call gqlFetch, must NOT emit console.error, and must
  // pass user=null / isAdmin=false to AppShell.
  test("AuthSessionMissingError: no gqlFetch call, no console output, AppShell gets user=null isAdmin=false", async () => {
    const noSession = new Error("Auth session missing!");
    noSession.name = "AuthSessionMissingError";
    setMockSupabaseUserError(noSession);

    const props = await renderLayoutAndGetShellProps();

    expect(gqlFetch).not.toHaveBeenCalled();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case 2: A non-AuthSessionMissingError from getUser() (e.g. a transport
  // failure) must be logged with the [layout] prefix and degrade gracefully —
  // AppShell receives a null user and false isAdmin, no gqlFetch is called.
  test("non-AuthSessionMissingError from getUser: console.error with [layout] prefix, degraded shell, no gqlFetch", async () => {
    const transportError = new Error("network failure");
    transportError.name = "FetchError";
    setMockSupabaseUserError(transportError);

    const props = await renderLayoutAndGetShellProps();

    expect(gqlFetch).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[layout] getUser() failed:",
      transportError.name,
      transportError.message,
    );
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case 3: Authenticated user whose me query returns an admin role.
  // AppShell must receive user.email and isAdmin=true.
  // gqlFetch must be called with { revalidate: 0 }.
  test("authenticated user + admin role: AppShell gets user.email and isAdmin=true, gqlFetch called with revalidate:0", async () => {
    setMockSupabaseUser({ id: "u-3", email: "admin@example.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [{ name: "admin" }] },
    } as never);

    const props = await renderLayoutAndGetShellProps();

    // gqlFetch must be called with { revalidate: 0 } — never with the default
    // cache heuristic — because me data depends on the current user's auth token.
    expect(gqlFetch).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ revalidate: 0 }),
    );
    expect(props.user).toEqual({ email: "admin@example.com" });
    expect(props.isAdmin).toBe(true);
  });

  // Case 4: Authenticated user whose me query returns a non-admin role.
  test("authenticated user + non-admin role: isAdmin=false", async () => {
    setMockSupabaseUser({ id: "u-4", email: "member@example.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [{ name: "member" }] },
    } as never);

    const props = await renderLayoutAndGetShellProps();

    expect(props.isAdmin).toBe(false);
    expect(props.user).toEqual({ email: "member@example.com" });
  });

  // Case 5: Authenticated user whose me query returns empty roles.
  test("authenticated user + empty roles array: isAdmin=false", async () => {
    setMockSupabaseUser({ id: "u-5", email: "new@example.com" });
    vi.mocked(gqlFetch).mockResolvedValueOnce({
      me: { roles: [] },
    } as never);

    const props = await renderLayoutAndGetShellProps();

    expect(props.isAdmin).toBe(false);
  });

  // Case 6: Authenticated user but gqlFetch rejects with UNAUTHENTICATED.
  // Expected race during clock skew / JWKS rotation — must degrade SILENTLY
  // (no console.warn, no console.error), isAdmin=false.
  test("authenticated user + UNAUTHENTICATED from gqlFetch: silent degradation, isAdmin=false, no console output", async () => {
    setMockSupabaseUser({ id: "u-6", email: "user@example.com" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(makeUnauthenticatedError());

    const props = await renderLayoutAndGetShellProps();

    // UNAUTHENTICATED is a known race — must not surface to the operator log.
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(props.isAdmin).toBe(false);
    // user.email is still available even when the me query fails.
    expect(props.user).toEqual({ email: "user@example.com" });
  });

  // Case 7: Authenticated user but gqlFetch rejects with an unexpected error
  // (not UNAUTHENTICATED). The layout must:
  //   - emit console.warn with "[layout] me query unexpectedly failed"
  //   - include a structured payload with a discriminating key (user_id) so
  //     the assertion cannot be satisfied by a bare Error instance — per
  //     .claude/rules/frontend-typescript-conventions.md "expect.objectContaining
  //     ({ message }) is not enough"
  //   - NOT include email or display_name in the payload (PII protection) — per
  //     .claude/rules/error-wrapping.md "Assert PII absence on log lines that
  //     carry user_id"
  //   - degrade to isAdmin=false without throwing
  test("authenticated user + unexpected gqlFetch error: console.warn with discriminating payload, PII absent, isAdmin=false", async () => {
    const userId = "u-7";
    setMockSupabaseUser({ id: userId, email: "private@example.com" });
    vi.mocked(gqlFetch).mockRejectedValueOnce(new Error("transport boom"));

    const props = await renderLayoutAndGetShellProps();

    expect(consoleErrorSpy).not.toHaveBeenCalled();

    // The warn must be called with the exact [layout] scope prefix.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[layout] me query unexpectedly failed",
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
    // only the me query fails.
    expect(props.user).toEqual({ email: "private@example.com" });
  });

  // Case 8: Anonymous user — user is null with no error (e.g. a clean
  // signed-out state). The layout must short-circuit the me query and pass
  // null / false to AppShell.
  test("anonymous user (user=null, no error): no gqlFetch call, AppShell gets user=null isAdmin=false", async () => {
    setMockSupabaseUser(null); // explicit for readability; also the default after reset

    const props = await renderLayoutAndGetShellProps();

    expect(gqlFetch).not.toHaveBeenCalled();
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(consoleWarnSpy).not.toHaveBeenCalled();
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });
});
