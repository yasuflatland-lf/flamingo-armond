// @vitest-environment happy-dom

import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// next/navigation — redirect throws so the RSC aborts the same way
// Next.js's server runtime does.
const mockRedirect = vi.fn((path: string) => {
  throw new Error(`REDIRECT:${path}`);
});
vi.mock("next/navigation", () => ({
  redirect: (path: string) => mockRedirect(path),
}));

// next/headers — default: authenticated. Individual tests can override with
// mockResolvedValueOnce.
vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ "x-auth-status": "authenticated" })),
}));

// Stub ChangeEmailClient — avoids pulling in Apollo/browser deps.
vi.mock("./change-email-client", () => ({
  ChangeEmailClient: ({ currentEmail }: { currentEmail: string | null }) => (
    <div data-testid="change-email-client" data-current-email={currentEmail ?? ""}>
      ChangeEmailClient
    </div>
  ),
}));

// next-intl/server — the page resolves the Profile namespace via getTranslations.
// Back the mock with createTranslator + the real en catalog so t() produces the
// original English heading copy, keeping any string assertions valid.
vi.mock("next-intl/server", () => ({
  getTranslations: vi.fn(async (namespace: "Profile") =>
    createTranslator({ locale: "en", messages: enMessages, namespace }),
  ),
}));

import { headers } from "next/headers";
import { createTranslator } from "next-intl";
import enMessages from "../../../../messages/en.json";
import ChangeEmailPage from "./page";

describe("ChangeEmailPage — auth gate", () => {
  beforeEach(() => {
    mockRedirect.mockClear();
  });

  it("redirects to /login when not authenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(new Headers({ "x-auth-status": "anonymous" }));

    await expect(ChangeEmailPage()).rejects.toThrow("REDIRECT:/login");

    expect(mockRedirect).toHaveBeenCalledWith("/login");
  });

  it("renders with forwarded email when authenticated", async () => {
    vi.mocked(headers).mockResolvedValueOnce(
      new Headers({ "x-auth-status": "authenticated", "x-user-email": "alice@example.com" }),
    );

    const jsx = await ChangeEmailPage();
    render(jsx);

    const stub = screen.getByTestId("change-email-client");
    expect(stub).toHaveAttribute("data-current-email", "alice@example.com");
  });
});
