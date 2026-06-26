// @vitest-environment happy-dom
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

// Stub AdminUsersClient — it is a "use client" component that requires an
// ApolloProvider. The RSC page test only needs to verify the auth gate.
vi.mock("./admin-users-client", () => ({
  AdminUsersClient: () => <div data-testid="admin-users-client" />,
}));

import { render, screen } from "@testing-library/react";
import { headers } from "next/headers";
import AdminUsersPage from "./page";

describe("AdminUsersPage — auth gate", () => {
  it("redirects to / when x-auth-status is anonymous", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));

    await expect(AdminUsersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("redirects to / when x-auth-status is stale", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "stale" }));

    await expect(AdminUsersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("redirects to / when x-auth-status is error", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "error" }));

    await expect(AdminUsersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("redirects to / when x-auth-status header is absent", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers());

    await expect(AdminUsersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("renders AdminUsersClient when authenticated", async () => {
    const jsx = await AdminUsersPage();
    render(jsx);

    expect(screen.getByTestId("admin-users-client")).toBeInTheDocument();
  });
});
