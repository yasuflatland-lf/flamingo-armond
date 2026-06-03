import { describe, expect, test, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any import of the module under test.
// ---------------------------------------------------------------------------

// AuthShell is opaque to this test; we only verify that the correct elements
// appear in (or are absent from) the returned JSX tree, and that the forwarded
// identity props reach it. Returning props as the element lets findElement
// locate it by type.name and lets assertions read its props directly.
// Named functions are required so findElement can locate them by type.name.
vi.mock("@/components/auth-shell", () => ({
  AuthShell: function AuthShell(props: {
    user?: { email: string | null } | null;
    isAdmin?: boolean;
    children?: React.ReactNode;
  }) {
    return props as unknown as React.ReactElement;
  },
}));

vi.mock("@/app/providers", () => ({
  Providers: function Providers(props: { children?: React.ReactNode; nonce: string | undefined }) {
    return props as unknown as React.ReactElement;
  },
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
// the layout can decide whether to mount AuthShell. Tests override the return
// value per case.
const mockGetHeader = vi.fn<(name: string) => string | null>(() => null);
vi.mock("next/headers", () => ({
  headers: vi.fn(() => Promise.resolve({ get: mockGetHeader })),
}));

// PWA components — stubs to prevent transitive import errors.
vi.mock("@/components/pwa/sw-register", () => ({
  SwRegister: () => null,
}));
vi.mock("@/components/pwa/apple-install-hint", () => ({
  AppleInstallHint: () => null,
}));
vi.mock("@vercel/speed-insights/next", () => ({
  SpeedInsights: () => null,
}));

// ---------------------------------------------------------------------------
// Import after mocks are registered.
// ---------------------------------------------------------------------------

import RootLayout, { viewport } from "@/app/layout";

// ---------------------------------------------------------------------------
// Helper: recursive element finder
//
// Traverses a React element tree and returns the first element that satisfies
// the predicate. Recurses into `props.children` (array or single).
// ---------------------------------------------------------------------------

type ReactElementLike = {
  type: unknown;
  props?: {
    children?: unknown;
    nonce?: unknown;
    [key: string]: unknown;
  };
};

function findElement(
  node: unknown,
  predicate: (el: ReactElementLike) => boolean,
): ReactElementLike | null {
  if (node == null || typeof node !== "object") return null;
  const el = node as ReactElementLike;

  if ("type" in el && predicate(el)) return el;

  const props = el.props;
  if (props == null) return null;

  // Check children (array or single).
  const { children } = props;
  if (Array.isArray(children)) {
    for (const child of children) {
      const found = findElement(child, predicate);
      if (found != null) return found;
    }
  } else if (children != null) {
    const found = findElement(children, predicate);
    if (found != null) return found;
  }

  return null;
}

/** Returns true when the element's type.name matches the given component name. */
function byName(name: string): (el: ReactElementLike) => boolean {
  return (el) => {
    const t = el.type;
    if (typeof t === "function") {
      return (t as { name?: string }).name === name;
    }
    return false;
  };
}

// ---------------------------------------------------------------------------
// Test suite: root layout structural routing
// ---------------------------------------------------------------------------

describe("RootLayout — structural branch selection", () => {
  // Case: default route (x-pathname absent / "/") — the non-bypass branch.
  // The returned tree must mount AuthShell directly (no async boundary) and
  // forward the layout's children to it. With no identity headers set,
  // readAuthContext classifies the request as anonymous, so AuthShell receives
  // user={null} isAdmin={false}.
  test("default route: AuthShell is mounted with the layout's children and anonymous identity", async () => {
    // x-pathname absent; mockGetHeader returns null by default.
    mockGetHeader.mockReturnValue(null);

    const tree = await RootLayout({ children: <div /> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).not.toBeNull();

    // AuthShell receives the layout's children as its own children prop.
    expect(authShellEl?.props?.children).toBeDefined();

    // Anonymous classification: null user, no admin hint.
    expect(authShellEl?.props?.user).toBeNull();
    expect(authShellEl?.props?.isAdmin).toBe(false);
  });

  // Case: default route — the middleware-forwarded identity headers reach
  // AuthShell as typed props (the source of the navigation shell's display
  // identity and admin nav hint).
  test("passes middleware-forwarded identity into AuthShell", async () => {
    const headerMap: Record<string, string> = {
      "x-pathname": "/cardgroups",
      "x-auth-status": "authenticated",
      "x-user-email": "a@b.c",
      "x-user-is-admin": "true",
    };
    mockGetHeader.mockImplementation((name: string) => headerMap[name] ?? null);

    const tree = await RootLayout({ children: <p>child</p> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).not.toBeNull();
    const props = authShellEl?.props as { user: { email: string | null }; isAdmin: boolean };
    expect(props.user.email).toBe("a@b.c");
    expect(props.isAdmin).toBe(true);
  });

  // Case: /login route — the bypass branch. No AuthShell; children render
  // directly inside Providers.
  test("/login: layout renders bare children without AuthShell", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-pathname" ? "/login" : null));

    const tree = await RootLayout({ children: <div data-testid="login-children" /> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).toBeNull();
  });

  // Case: /onboarding route — same bare-shell early return as /login.
  // No AuthShell.
  test("/onboarding: layout renders bare children without AuthShell", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-pathname" ? "/onboarding" : null));

    const tree = await RootLayout({ children: <div data-testid="onboarding-children" /> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).toBeNull();
  });

  // Case: default route — nonce from x-nonce header is forwarded to Providers.
  test("default route: nonce from x-nonce header is forwarded to Providers", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-nonce" ? "test-nonce-value" : null));
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBe("test-nonce-value");
  });

  // Case: bypass routes (/login, /onboarding) — nonce from x-nonce header is forwarded to Providers.
  test.each([
    "/login",
    "/onboarding",
  ])("%s: nonce from x-nonce header is forwarded to Providers", async (pathname: string) => {
    mockGetHeader.mockImplementation((name) => {
      if (name === "x-pathname") return pathname;
      if (name === "x-nonce") return "test-nonce-value";
      return null;
    });
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBe("test-nonce-value");
  });

  // Case: stale/error status — the layout must not expose user or isAdmin even
  // when the x-user-is-admin header is "true". The gate is status === "authenticated"
  // only; all other statuses degrade to anonymous shell regardless of the header.
  test.each([
    "stale",
    "error",
  ])("%s status yields an anonymous shell even if x-user-is-admin is true", async (authStatus: string) => {
    const headerMap: Record<string, string> = {
      "x-pathname": "/cardgroups",
      "x-auth-status": authStatus,
      "x-user-email": "a@b.c",
      "x-user-is-admin": "true",
    };
    mockGetHeader.mockImplementation((name: string) => headerMap[name] ?? null);

    const tree = await RootLayout({ children: <p>child</p> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).not.toBeNull();
    const props = authShellEl?.props as { user: { email: string | null } | null; isAdmin: boolean };
    expect(props.user).toBeNull();
    expect(props.isAdmin).toBe(false);
  });

  // Case: absent x-nonce header — nonce is undefined at Providers.
  test("default route: absent x-nonce header passes undefined nonce to Providers", async () => {
    mockGetHeader.mockReturnValue(null);
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBeUndefined();
  });
});

describe("viewport metadata", () => {
  // Pins the iOS dark-mode black-flash fix: a light-only app must declare
  // color-scheme: light so the standalone PWA canvas stays light on dark devices.
  test("pins the scheme to light so a dark-mode iOS PWA does not flash a black canvas", () => {
    expect(viewport.colorScheme).toBe("light");
  });

  test("keeps the brand theme color", () => {
    expect(viewport.themeColor).toBe("#FF6F79");
  });
});
