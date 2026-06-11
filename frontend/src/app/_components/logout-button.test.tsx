// @vitest-environment jsdom

import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";

const mockReplace = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace, refresh: mockRefresh }),
}));

const mockSignOut = vi.fn();
vi.mock("@/lib/supabase/client", () => ({
  createSupabaseBrowserClient: () => ({
    auth: { signOut: () => mockSignOut() },
  }),
}));

import { LogoutButton } from "./logout-button";

beforeEach(() => {
  mockReplace.mockReset();
  mockRefresh.mockReset();
  mockSignOut.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<LogoutButton>", () => {
  it("on success navigates to /login before refreshing the route", async () => {
    mockSignOut.mockResolvedValue({ error: null });
    const callOrder: string[] = [];
    mockReplace.mockImplementation((path: string) => callOrder.push(`replace:${path}`));
    mockRefresh.mockImplementation(() => callOrder.push("refresh"));

    renderWithIntl(<LogoutButton />);
    await userEvent.click(screen.getByRole("button", { name: /logout/i }));

    // replace must precede refresh: refreshing on the old (protected) URL is
    // what causes the anonymous Header to flash a "Sign in" link before the
    // page-level redirect kicks in.
    expect(callOrder).toEqual(["replace:/login", "refresh"]);
  });

  it("does not navigate or refresh when signOut errors", async () => {
    mockSignOut.mockResolvedValue({ error: { message: "boom" } });
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    renderWithIntl(<LogoutButton />);
    await userEvent.click(screen.getByRole("button", { name: /logout/i }));

    expect(mockReplace).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(consoleErrorSpy).toHaveBeenCalledWith("[logout] signOut failed:", "boom");
  });
});
