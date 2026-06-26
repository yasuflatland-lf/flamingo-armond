// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

// usePathname drives the shell/bare decision. Override per test.
const mockUsePathname = vi.fn<() => string>();
vi.mock("next/navigation", () => ({
  usePathname: () => mockUsePathname(),
}));

// AuthShell is opaque here — we only assert presence/absence and the forwarded
// identity props. Render a sentinel element carrying the props as data-attrs.
vi.mock("./auth-shell", () => ({
  AuthShell: ({
    user,
    isAdmin,
    children,
  }: {
    user: { email: string | null } | null;
    isAdmin: boolean;
    children: React.ReactNode;
  }) => (
    <div data-testid="auth-shell" data-email={user?.email ?? ""} data-is-admin={String(isAdmin)}>
      {children}
    </div>
  ),
}));

vi.mock("./pwa/apple-install-hint", () => ({
  AppleInstallHint: () => <div data-testid="apple-install-hint" />,
}));

import { ConditionalShell } from "./conditional-shell";

afterEach(() => {
  vi.clearAllMocks();
});

describe("<ConditionalShell>", () => {
  describe("bare routes (/, /login, /onboarding, /terms, /privacy)", () => {
    it.each([
      "/",
      "/login",
      "/onboarding",
      "/onboarding/start",
      "/terms",
      "/privacy",
    ])("renders children directly without the navigation shell on %s", (pathname) => {
      mockUsePathname.mockReturnValue(pathname);
      render(
        <ConditionalShell user={null} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.getByTestId("page")).toBeInTheDocument();
      expect(screen.queryByTestId("auth-shell")).toBeNull();
      expect(screen.queryByTestId("apple-install-hint")).toBeNull();
    });

    it.each([
      "/",
      "/login",
      "/onboarding",
      "/onboarding/start",
    ])("keeps the shell hidden on %s even for an authenticated identity", (pathname) => {
      // A soft navigation can reach a bare route while the layout-computed
      // identity is still authenticated; the shell must stay hidden regardless
      // of identity. /onboarding and /onboarding/start are both reached WHILE
      // authenticated (the display-name gate and the first-deck chooser), and `/`
      // is the post-login redirect-only dispatcher reached WHILE authenticated —
      // mounting the shell there flashes the nav rail before `/` redirects (the
      // first-login `/` → `/onboarding` rail flash). The authenticated case is the
      // one that matters for all three.
      mockUsePathname.mockReturnValue(pathname);
      render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={true}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.queryByTestId("auth-shell")).toBeNull();
    });
  });

  // Regression lock for the onboarding-404 nav-rail bug: an unknown path renders
  // app/not-found.tsx under the arbitrary pathname the user typed. Because the
  // shell is gated by a content-route allowlist (SHELL_ROUTE_PREFIXES), every
  // unknown path is bare — so a not-yet-onboarded user who mistypes a URL and
  // lands on the 404 page never sees the nav rail. "/cardgroupsX" pins the `/`
  // prefix boundary: a near-miss must NOT match the /cardgroups content route.
  describe("unknown routes (404) render bare", () => {
    it.each([
      "/this-route-does-not-exist",
      "/foobar",
      "/some/deep/unknown/path",
      "/cardgroupsX",
    ])("renders children directly without the navigation shell on %s", (pathname) => {
      mockUsePathname.mockReturnValue(pathname);
      render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.getByTestId("page")).toBeInTheDocument();
      expect(screen.queryByTestId("auth-shell")).toBeNull();
      expect(screen.queryByTestId("apple-install-hint")).toBeNull();
    });
  });

  describe("full-shell routes", () => {
    it.each([
      "/cardgroups",
      "/cardgroups/123",
      "/cards",
      "/cards/new",
      "/catalog",
      "/learn/123",
      "/profile",
      "/admin/users",
    ])("mounts the navigation shell and install hint on %s", (pathname) => {
      mockUsePathname.mockReturnValue(pathname);
      render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.getByTestId("auth-shell")).toBeInTheDocument();
      expect(screen.getByTestId("page")).toBeInTheDocument();
      expect(screen.getByTestId("apple-install-hint")).toBeInTheDocument();
    });

    it("forwards user and isAdmin to AuthShell", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      render(
        <ConditionalShell user={{ email: "admin@b.c" }} isAdmin={true}>
          <div />
        </ConditionalShell>,
      );

      const shell = screen.getByTestId("auth-shell");
      expect(shell.getAttribute("data-email")).toBe("admin@b.c");
      expect(shell.getAttribute("data-is-admin")).toBe("true");
    });

    it("renders the anonymous shell when user is null", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      render(
        <ConditionalShell user={null} isAdmin={false}>
          <div />
        </ConditionalShell>,
      );

      const shell = screen.getByTestId("auth-shell");
      expect(shell.getAttribute("data-email")).toBe("");
      expect(shell.getAttribute("data-is-admin")).toBe("false");
    });
  });

  // The actual bug: the shell decision must REACT to a client-side pathname
  // change, not freeze at initial render. The root layout is a server component
  // that does not re-render on soft navigations, so the shell lingered over
  // /login when reached via the /terms-404 → "Back to Home" → / → /login chain.
  // usePathname() re-renders ConditionalShell on every navigation; these tests
  // assert the shell appears/disappears across a simulated navigation. A future
  // change that hoists the route decision out of the render cycle (memoization,
  // a module constant, or a move back to the server) fails here.
  describe("reacts to a client-side navigation (regression lock)", () => {
    it("drops the shell when navigating from a full route to a bare route", () => {
      mockUsePathname.mockReturnValue("/cardgroups");
      const { rerender } = render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );
      expect(screen.getByTestId("auth-shell")).toBeInTheDocument();

      // Simulate a soft navigation to /login: usePathname() now returns /login.
      mockUsePathname.mockReturnValue("/login");
      rerender(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.queryByTestId("auth-shell")).toBeNull();
      expect(screen.getByTestId("page")).toBeInTheDocument();
    });

    it("mounts the shell when navigating from a bare route to a full route", () => {
      mockUsePathname.mockReturnValue("/login");
      const { rerender } = render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );
      expect(screen.queryByTestId("auth-shell")).toBeNull();

      mockUsePathname.mockReturnValue("/cardgroups");
      rerender(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.getByTestId("auth-shell")).toBeInTheDocument();
    });
  });
});
