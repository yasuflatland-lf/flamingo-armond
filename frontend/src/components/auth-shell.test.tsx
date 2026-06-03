// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AuthShell } from "./auth-shell";

vi.mock("@/components/nav/app-shell", () => ({
  AppShell: ({
    user,
    isAdmin,
    children,
  }: {
    user: { email: string | null } | null;
    isAdmin: boolean;
    children: React.ReactNode;
  }) => (
    <div data-testid="app-shell" data-email={user?.email ?? ""} data-is-admin={String(isAdmin)}>
      {children}
    </div>
  ),
}));

vi.mock("@/components/ui/sonner", () => ({ Toaster: () => <div data-testid="toaster" /> }));

describe("AuthShell", () => {
  it("passes user and isAdmin through to AppShell and renders children + Toaster", () => {
    render(
      <AuthShell user={{ email: "a@b.c" }} isAdmin>
        <p>child</p>
      </AuthShell>,
    );
    const shell = screen.getByTestId("app-shell");
    expect(shell.getAttribute("data-email")).toBe("a@b.c");
    expect(shell.getAttribute("data-is-admin")).toBe("true");
    expect(screen.getByText("child")).toBeInTheDocument();
    expect(screen.getByTestId("toaster")).toBeInTheDocument();
  });

  it("renders an anonymous shell when user is null", () => {
    render(
      <AuthShell user={null} isAdmin={false}>
        <p>child</p>
      </AuthShell>,
    );
    const shell = screen.getByTestId("app-shell");
    expect(shell.getAttribute("data-email")).toBe("");
    expect(shell.getAttribute("data-is-admin")).toBe("false");
  });
});
