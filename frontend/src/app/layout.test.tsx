import { Suspense } from "react";
import { describe, expect, test, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any import of the module under test.
// ---------------------------------------------------------------------------

// AuthShell and BootSplash are opaque to this test; we only verify that the
// correct elements appear in (or are absent from) the returned JSX tree.
// Named functions are required so findElement can locate them by type.name.
vi.mock("@/components/auth-shell", () => ({
  AuthShell: function AuthShell(props: { children?: React.ReactNode }) {
    return props as unknown as React.ReactElement;
  },
}));

vi.mock("@/components/boot-splash", () => ({
  BootSplash: function BootSplash() {
    return null;
  },
}));

vi.mock("@/app/providers", () => ({
  Providers: function Providers(props: { children?: React.ReactNode; nonce?: string }) {
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
// the predicate. Recurses into both `props.children` (array or single) AND
// `props.fallback` so Suspense boundaries are fully covered.
// ---------------------------------------------------------------------------

type ReactElementLike = {
  type: unknown;
  props?: {
    children?: unknown;
    fallback?: unknown;
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

  // Check fallback first (covers Suspense.fallback subtree).
  if (props.fallback != null) {
    const found = findElement(props.fallback, predicate);
    if (found != null) return found;
  }

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

/** Returns true when the element's type is the React.Suspense symbol. */
function isSuspense(el: ReactElementLike): boolean {
  return el.type === Suspense;
}

// ---------------------------------------------------------------------------
// Test suite: root layout structural routing
// ---------------------------------------------------------------------------

describe("RootLayout — structural branch selection", () => {
  // Case: default route (x-pathname absent / "/") — the non-bypass branch.
  // The returned tree must contain a Suspense element whose fallback is a
  // BootSplash element and whose child is an AuthShell element.
  test("default route: Suspense wraps AuthShell with BootSplash as fallback", async () => {
    // x-pathname absent; mockGetHeader returns null by default.
    mockGetHeader.mockReturnValue(null);

    const tree = await RootLayout({ children: <div /> });

    // (1) A Suspense element exists somewhere in the tree.
    const suspenseEl = findElement(tree, isSuspense);
    expect(suspenseEl).not.toBeNull();

    // (2) The Suspense fallback is a BootSplash element.
    const fallbackEl = suspenseEl?.props?.fallback;
    expect(fallbackEl).not.toBeNull();
    const bootSplashEl = findElement(fallbackEl, byName("BootSplash"));
    expect(bootSplashEl).not.toBeNull();

    // (3) AuthShell is reachable inside the Suspense child.
    const suspenseChildren = suspenseEl?.props?.children;
    const authShellEl = findElement(suspenseChildren, byName("AuthShell"));
    expect(authShellEl).not.toBeNull();

    // (4) AuthShell receives the layout's children as its own children prop.
    expect(authShellEl?.props?.children).toBeDefined();
  });

  // Case: /login route — the bypass branch. No AuthShell, no Suspense,
  // no BootSplash. Children render directly inside Providers.
  test("/login: layout renders bare children without AuthShell or BootSplash", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-pathname" ? "/login" : null));

    const tree = await RootLayout({ children: <div data-testid="login-children" /> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).toBeNull();

    const bootSplashEl = findElement(tree, byName("BootSplash"));
    expect(bootSplashEl).toBeNull();

    const suspenseEl = findElement(tree, isSuspense);
    expect(suspenseEl).toBeNull();
  });

  // Case: /onboarding route — same bare-shell early return as /login.
  // No AuthShell, no Suspense, no BootSplash.
  test("/onboarding: layout renders bare children without AuthShell or BootSplash", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-pathname" ? "/onboarding" : null));

    const tree = await RootLayout({ children: <div data-testid="onboarding-children" /> });

    const authShellEl = findElement(tree, byName("AuthShell"));
    expect(authShellEl).toBeNull();

    const bootSplashEl = findElement(tree, byName("BootSplash"));
    expect(bootSplashEl).toBeNull();

    const suspenseEl = findElement(tree, isSuspense);
    expect(suspenseEl).toBeNull();
  });

  // Test A — default route forwards nonce.
  test("default route: nonce from x-nonce header is forwarded to Providers", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-nonce" ? "test-nonce-value" : null));
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBe("test-nonce-value");
  });

  // Test B — /login bypass route forwards nonce.
  test("/login: nonce from x-nonce header is forwarded to Providers", async () => {
    mockGetHeader.mockImplementation((name) => {
      if (name === "x-pathname") return "/login";
      if (name === "x-nonce") return "test-nonce-value";
      return null;
    });
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBe("test-nonce-value");
  });

  // Test C — absent x-nonce → undefined.
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
