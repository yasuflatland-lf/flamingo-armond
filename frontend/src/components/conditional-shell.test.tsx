// @vitest-environment jsdom
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
  describe("bare routes (/login, /onboarding)", () => {
    it.each(["/login", "/onboarding"])(
      "renders children directly without the navigation shell on %s",
      (pathname) => {
        mockUsePathname.mockReturnValue(pathname);
        render(
          <ConditionalShell user={null} isAdmin={false}>
            <div data-testid="page" />
          </ConditionalShell>,
        );

        expect(screen.getByTestId("page")).toBeInTheDocument();
        expect(screen.queryByTestId("auth-shell")).toBeNull();
        expect(screen.queryByTestId("apple-install-hint")).toBeNull();
      },
    );

    it("keeps the shell hidden on /login even for an authenticated identity", () => {
      // A soft navigation can reach /login while the layout-computed identity is
      // still authenticated; the shell must stay hidden regardless of identity.
      mockUsePathname.mockReturnValue("/login");
      render(
        <ConditionalShell user={{ email: "a@b.c" }} isAdmin={true}>
          <div data-testid="page" />
        </ConditionalShell>,
      );

      expect(screen.queryByTestId("auth-shell")).toBeNull();
    });
  });

  describe("full-shell routes", () => {
    it.each(["/", "/cardgroups", "/cardgroups/123", "/terms", "/admin/users"])(
      "mounts the navigation shell and install hint on %s",
      (pathname) => {
        mockUsePathname.mockReturnValue(pathname);
        render(
          <ConditionalShell user={{ email: "a@b.c" }} isAdmin={false}>
            <div data-testid="page" />
          </ConditionalShell>,
        );

        expect(screen.getByTestId("auth-shell")).toBeInTheDocument();
        expect(screen.getByTestId("page")).toBeInTheDocument();
        expect(screen.getByTestId("apple-install-hint")).toBeInTheDocument();
      },
    );

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
});
