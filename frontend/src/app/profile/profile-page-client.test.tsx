// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateLearnDisplayModeDocument, UpdateProfileDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { ProfilePageClient } from "./profile-page-client";

const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockRefresh = vi.fn();
let mockSearchParamsValue = "";

vi.mock("next/navigation", () => ({
  usePathname: () => "/profile",
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
    refresh: mockRefresh,
  }),
  useSearchParams: () => new URLSearchParams(mockSearchParamsValue),
}));

beforeEach(() => {
  mockPush.mockReset();
  mockReplace.mockReset();
  mockRefresh.mockReset();
  mockSearchParamsValue = "";
});

function makeUpdateProfileMock(variables: { input: { displayName: string; bio?: string | null } }) {
  return {
    request: {
      query: UpdateProfileDocument,
      variables,
    },
    result: {
      data: {
        updateProfile: {
          __typename: "UpdateProfileSuccess" as const,
          user: {
            __typename: "User" as const,
            id: "user-1",
            displayName: variables.input.displayName,
            bio: variables.input.bio ?? null,
            avatarUrl: null,
          },
        },
      },
    },
  };
}

function makeUpdateLearnDisplayModeMock(
  mode: "ALWAYS_VISIBLE" | "FLIP_TO_REVEAL",
  onCalled?: () => void,
) {
  return {
    request: {
      query: UpdateLearnDisplayModeDocument,
      variables: { mode },
    },
    result: () => {
      onCalled?.();
      return {
        data: {
          updateLearnDisplayMode: {
            __typename: "User" as const,
            id: "user-1",
            learnDisplayMode: mode,
          },
        },
      };
    },
  };
}

describe("<ProfilePageClient>", () => {
  const initial = { displayName: "Alice", bio: "hello" };

  it("Edit profile button pushes ?edit=self", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /edit profile/i }));

    expect(mockPush).toHaveBeenCalledWith("/profile?edit=self", { scroll: false });
  });

  it("renders the Settings section with the language switcher", () => {
    mockSearchParamsValue = "";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { name: /settings/i })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /language/i })).toBeInTheDocument();
  });

  it("opens the sheet when ?edit=self (singleton sentinel) is present in the query", () => {
    mockSearchParamsValue = "edit=self";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { name: /edit profile/i })).toBeInTheDocument();
  });

  it("successful save closes the sheet, replaces to /profile, and refreshes", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=self";
    const mocks = [makeUpdateProfileMock({ input: { displayName: "Alice", bio: "hello" } })];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/profile", { scroll: false });
      expect(mockRefresh).toHaveBeenCalledTimes(1);
    });
  });

  it("Change email closes the sheet URL before navigating to the email-change route", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=self";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("link", { name: /change email/i }));

    expect(mockReplace).toHaveBeenCalledWith("/profile", { scroll: false });
    expect(mockPush).toHaveBeenCalledWith("/profile/change-email");
  });

  it("Change email does not navigate while the profile save is submitting", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=self";
    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: "hello" } },
        },
        // Long delay keeps the mutation pending throughout the test assertions
        // so `submitting` stays `true` while we click Change email. A short
        // delay races the user-event click chain on slow CI and lets
        // `handleSaved` fire mid-click, calling `mockReplace` before the
        // assertion runs.
        delay: 30_000,
        result: {
          data: {
            updateProfile: {
              __typename: "UpdateProfileSuccess" as const,
              user: {
                __typename: "User" as const,
                id: "user-1",
                displayName: "Alice",
                bio: "hello",
                avatarUrl: null,
              },
            },
          },
        },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /^save$/i }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /saving/i })).toBeDisabled();
    });

    await user.click(screen.getByRole("link", { name: /change email/i }));

    expect(mockReplace).not.toHaveBeenCalled();
    expect(mockPush).not.toHaveBeenCalledWith("/profile/change-email");
  });

  it("clicking Always visible fires updateLearnDisplayMode with mode ALWAYS_VISIBLE", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "";
    const mutationCalled = vi.fn();
    const mocks = [makeUpdateLearnDisplayModeMock("ALWAYS_VISIBLE", mutationCalled)];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
        />
      </MockedProvider>,
    );

    // FLIP_TO_REVEAL is the active mode on mount.
    const flipOption = screen.getByTestId("display-mode-flip-to-reveal");
    expect(flipOption).toHaveAttribute("aria-pressed", "true");

    const alwaysVisibleOption = screen.getByTestId("display-mode-always-visible");
    expect(alwaysVisibleOption).toHaveAttribute("aria-pressed", "false");

    await user.click(alwaysVisibleOption);

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });
});
