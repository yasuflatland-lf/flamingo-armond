// @vitest-environment jsdom

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DeleteMyAccountDocument } from "@/generated/graphql";
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

import { DeleteAccountSection } from "./delete-account-section";

const CONFIRM_PHRASE = "delete my account";

function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

function successMock() {
  return {
    request: { query: DeleteMyAccountDocument },
    result: { data: { deleteMyAccount: true } },
  };
}

function errorMock(code: string) {
  return {
    request: { query: DeleteMyAccountDocument },
    error: makeCodedError(code),
  };
}

function renderSection(mocks: React.ComponentProps<typeof MockedProvider>["mocks"] = []) {
  renderWithIntl(
    <MockedProvider mocks={mocks}>
      <DeleteAccountSection />
    </MockedProvider>,
  );
}

beforeEach(() => {
  mockReplace.mockReset();
  mockRefresh.mockReset();
  mockSignOut.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("<DeleteAccountSection>", () => {
  it("renders the danger zone with a delete trigger", () => {
    renderSection();
    expect(screen.getByRole("heading", { name: /danger zone/i })).toBeInTheDocument();
    expect(screen.getByTestId("delete-account-trigger")).toBeInTheDocument();
  });

  it("gates the confirm button on typing the exact phrase", async () => {
    const user = userEvent.setup();
    renderSection();

    await user.click(screen.getByTestId("delete-account-trigger"));
    expect(screen.getByTestId("delete-account-confirm")).toBeDisabled();

    await user.type(screen.getByTestId("delete-account-confirm-input"), "delete");
    expect(screen.getByTestId("delete-account-confirm")).toBeDisabled();

    await user.clear(screen.getByTestId("delete-account-confirm-input"));
    await user.type(screen.getByTestId("delete-account-confirm-input"), CONFIRM_PHRASE);
    expect(screen.getByTestId("delete-account-confirm")).toBeEnabled();
  });

  it("on success signs out and redirects to /login", async () => {
    const user = userEvent.setup();
    mockSignOut.mockResolvedValue({ error: null });
    renderSection([successMock()]);

    await user.click(screen.getByTestId("delete-account-trigger"));
    await user.type(screen.getByTestId("delete-account-confirm-input"), CONFIRM_PHRASE);
    await user.click(screen.getByTestId("delete-account-confirm"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/login");
    });
    expect(mockSignOut).toHaveBeenCalledTimes(1);
    expect(mockRefresh).toHaveBeenCalledTimes(1);
  });

  it("still redirects when signOut fails (non-fatal)", async () => {
    const user = userEvent.setup();
    mockSignOut.mockResolvedValue({ error: { message: "network" } });
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSection([successMock()]);

    await user.click(screen.getByTestId("delete-account-trigger"));
    await user.type(screen.getByTestId("delete-account-confirm-input"), CONFIRM_PHRASE);
    await user.click(screen.getByTestId("delete-account-confirm"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/login");
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("signOut after deleteMyAccount failed"),
      "network",
    );
  });

  it("shows the last-admin copy and does not redirect on FORBIDDEN", async () => {
    const user = userEvent.setup();
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSection([errorMock("FORBIDDEN")]);

    await user.click(screen.getByTestId("delete-account-trigger"));
    await user.type(screen.getByTestId("delete-account-confirm-input"), CONFIRM_PHRASE);
    await user.click(screen.getByTestId("delete-account-confirm"));

    await waitFor(() => {
      expect(screen.getByTestId("delete-account-error")).toHaveTextContent(/last admin/i);
    });
    expect(mockReplace).not.toHaveBeenCalled();
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith(
      "[profile] deleteMyAccount rejected",
      expect.objectContaining({ codes: ["FORBIDDEN"] }),
    );
  });

  it("does not configure optimisticResponse for deleteMyAccount", () => {
    const source = readFileSync(
      join(process.cwd(), "src/app/profile/delete-account-section.tsx"),
      "utf8",
    );
    expect(source).not.toContain("optimisticResponse");
  });
});
