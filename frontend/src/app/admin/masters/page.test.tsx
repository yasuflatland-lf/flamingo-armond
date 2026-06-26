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

vi.mock("./admin-masters-client", () => ({
  AdminMastersClient: () => <div data-testid="admin-masters-client" />,
}));

import { render, screen } from "@testing-library/react";
import { headers } from "next/headers";
import AdminMastersPage from "./page";

describe("AdminMastersPage — auth gate", () => {
  it("redirects to / when x-auth-status is anonymous", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));
    await expect(AdminMastersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("redirects to / when x-auth-status header is absent", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers());
    await expect(AdminMastersPage()).rejects.toThrow("REDIRECT:/");
  });

  it("renders AdminMastersClient when authenticated", async () => {
    const jsx = await AdminMastersPage();
    render(jsx);
    expect(screen.getByTestId("admin-masters-client")).toBeInTheDocument();
  });
});
