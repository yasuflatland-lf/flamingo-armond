import { beforeAll, describe, expect, test, vi } from "vitest";

// ---------------------------------------------------------------------------
// Module mocks — must be declared before any import of the module under test.
// ---------------------------------------------------------------------------

// ConditionalShell is opaque to this test; we only verify that it is mounted and
// that the forwarded identity props reach it. The shell/bare routing decision it
// makes from usePathname() is exercised in conditional-shell.test.tsx. Returning
// props as the element lets findElement locate it by type.name and lets
// assertions read its props directly. Named functions are required so findElement
// can locate them by type.name.
vi.mock("@/components/conditional-shell", () => ({
  ConditionalShell: function ConditionalShell(props: {
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

// next/headers — middleware forwards identity via request headers, which the
// layout reads to compute the shell props. Tests override the return value per case.
const mockGetHeader = vi.fn<(name: string) => string | null>(() => null);
vi.mock("next/headers", () => ({
  headers: vi.fn(() => Promise.resolve({ get: mockGetHeader })),
}));

// PWA components — stubs to prevent transitive import errors.
vi.mock("@/components/pwa/sw-register", () => ({
  SwRegister: () => null,
}));
vi.mock("@vercel/speed-insights/next", () => ({
  SpeedInsights: () => null,
}));

// next/font/google — the font loader is a build-time transform that only runs
// under the Next.js pipeline; in vitest the module-level Inter() call in
// layout.tsx must be stubbed to return a font object carrying the `variable`
// className the layout applies to <html>.
vi.mock("next/font/google", () => ({
  Inter: () => ({ variable: "__inter_variable", className: "__inter" }),
}));

// next-intl — the layout reads the active locale and wraps the tree in the
// client provider. Stub both so RootLayout/generateMetadata run without the
// Next.js request context. The provider is a passthrough so findElement can
// still recurse into its children to locate ConditionalShell/Providers.
vi.mock("next-intl", () => ({
  NextIntlClientProvider: function NextIntlClientProvider(props: { children?: React.ReactNode }) {
    return props as unknown as React.ReactElement;
  },
}));
vi.mock("next-intl/server", () => ({
  getLocale: vi.fn(() => Promise.resolve("en")),
  getTranslations: vi.fn(() =>
    Promise.resolve((key: string) => (key === "description" ? "Swiping flashcard app." : key)),
  ),
}));

// ---------------------------------------------------------------------------
// Import after mocks are registered.
// ---------------------------------------------------------------------------

import { getLocale } from "next-intl/server";
import RootLayout, { generateMetadata, viewport } from "@/app/layout";

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

describe("RootLayout — shell mounting and identity forwarding", () => {
  // The layout no longer branches on the pathname — it always mounts
  // ConditionalShell, which makes the shell/bare decision client-side from
  // usePathname() (see conditional-shell.test.tsx). The layout's job is to
  // forward the layout's children and the middleware-resolved identity into it.
  test("mounts ConditionalShell with the layout's children and anonymous identity by default", async () => {
    // No identity headers set; readAuthContext classifies the request as anonymous.
    mockGetHeader.mockReturnValue(null);

    const tree = await RootLayout({ children: <div /> });

    const shellEl = findElement(tree, byName("ConditionalShell"));
    expect(shellEl).not.toBeNull();

    // ConditionalShell receives the layout's children as its own children prop.
    expect(shellEl?.props?.children).toBeDefined();

    // Anonymous classification: null user, no admin hint.
    expect(shellEl?.props?.user).toBeNull();
    expect(shellEl?.props?.isAdmin).toBe(false);
  });

  // The root <html> lang attribute must track the locale resolved by next-intl
  // (src/i18n/request.ts), not a hardcoded "en".
  test("sets <html lang> to the active locale", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");
    mockGetHeader.mockReturnValue(null);

    const tree = (await RootLayout({ children: <div /> })) as { props: { lang: string } };

    expect(tree.props.lang).toBe("ja");
  });

  // The middleware-forwarded identity headers reach ConditionalShell as typed
  // props (the source of the navigation shell's display identity and admin nav hint).
  test("passes middleware-forwarded identity into ConditionalShell", async () => {
    const headerMap: Record<string, string> = {
      "x-auth-status": "authenticated",
      "x-user-email": "a@b.c",
      "x-user-is-admin": "true",
    };
    mockGetHeader.mockImplementation((name: string) => headerMap[name] ?? null);

    const tree = await RootLayout({ children: <p>child</p> });

    const shellEl = findElement(tree, byName("ConditionalShell"));
    expect(shellEl).not.toBeNull();
    const props = shellEl?.props as { user: { email: string | null }; isAdmin: boolean };
    expect(props.user.email).toBe("a@b.c");
    expect(props.isAdmin).toBe(true);
  });

  // Case: nonce from x-nonce header is forwarded to Providers.
  test("nonce from x-nonce header is forwarded to Providers", async () => {
    mockGetHeader.mockImplementation((name) => (name === "x-nonce" ? "test-nonce-value" : null));
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBe("test-nonce-value");
  });

  // Case: stale/error status — the layout must not expose user or isAdmin even
  // when the x-user-is-admin header is "true". The gate is status === "authenticated"
  // only; all other statuses degrade to anonymous shell regardless of the header.
  test.each(["stale", "error"])(
    "%s status yields an anonymous shell even if x-user-is-admin is true",
    async (authStatus: string) => {
      const headerMap: Record<string, string> = {
        "x-auth-status": authStatus,
        "x-user-email": "a@b.c",
        "x-user-is-admin": "true",
      };
      mockGetHeader.mockImplementation((name: string) => headerMap[name] ?? null);

      const tree = await RootLayout({ children: <p>child</p> });

      const shellEl = findElement(tree, byName("ConditionalShell"));
      expect(shellEl).not.toBeNull();
      const props = shellEl?.props as { user: { email: string | null } | null; isAdmin: boolean };
      expect(props.user).toBeNull();
      expect(props.isAdmin).toBe(false);
    },
  );

  // Case: absent x-nonce header — nonce is undefined at Providers.
  test("absent x-nonce header passes undefined nonce to Providers", async () => {
    mockGetHeader.mockReturnValue(null);
    const tree = await RootLayout({ children: <div /> });
    const providersEl = findElement(tree, byName("Providers"));
    expect(providersEl).not.toBeNull();
    expect(providersEl?.props?.nonce).toBeUndefined();
  });
});

describe("root metadata", () => {
  // metadata is now produced by the async generateMetadata (it reads the active
  // locale + Meta.description via next-intl), so resolve it once before the
  // assertions below.
  let metadata: Awaited<ReturnType<typeof generateMetadata>>;
  beforeAll(async () => {
    metadata = await generateMetadata();
  });

  // Pins the SEO surface: metadataBase resolves relative OG/canonical URLs,
  // the title template appends the site name to per-page titles, and the
  // canonical alternate marks the public landing surface as the only
  // canonical route of this auth-gated app.
  test("resolves metadataBase from NEXT_PUBLIC_SITE_URL (localhost default in tests)", () => {
    expect(metadata.metadataBase).toBeInstanceOf(URL);
    expect(String(metadata.metadataBase)).toBe("http://localhost:3000/");
  });

  test("keeps the default title and appends the site name via the template", () => {
    expect(metadata.title).toMatchObject({
      default: "Flamingo Armond",
      template: "%s | Flamingo Armond",
    });
  });

  test("declares the canonical alternate for the public landing surface", () => {
    expect(metadata.alternates?.canonical).toBe("/");
  });

  test("carries the Open Graph fields with the generated OG image", () => {
    expect(metadata.openGraph).toMatchObject({
      type: "website",
      siteName: "Flamingo Armond",
      title: "Flamingo Armond",
      description: "Swiping flashcard app.",
      url: "/",
      locale: "en_US",
      images: ["/opengraph-image"],
    });
  });

  test("maps the active locale to its Open Graph locale (ja -> ja_JP)", async () => {
    vi.mocked(getLocale).mockResolvedValueOnce("ja");
    const jaMetadata = await generateMetadata();
    expect(jaMetadata.openGraph).toMatchObject({ locale: "ja_JP" });
  });

  test("carries the Twitter Card fields with the generated OG image", () => {
    expect(metadata.twitter).toMatchObject({
      card: "summary_large_image",
      title: "Flamingo Armond",
      description: "Swiping flashcard app.",
      images: ["/opengraph-image"],
    });
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
