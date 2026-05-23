// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
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

describe("<ProfilePageClient>", () => {
  const initial = { displayName: "Alice", bio: "hello" };

  it("Edit profile button pushes ?edit=self", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "";

    render(
      <MockedProvider mocks={[]}>
        <ProfilePageClient email="alice@example.com" initial={initial} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /edit profile/i }));

    expect(mockPush).toHaveBeenCalledWith("/profile?edit=self", { scroll: false });
  });

  it("opens the sheet when ?edit=self (singleton sentinel) is present in the query", () => {
    mockSearchParamsValue = "edit=self";

    render(
      <MockedProvider mocks={[]}>
        <ProfilePageClient email="alice@example.com" initial={initial} />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { name: /edit profile/i })).toBeInTheDocument();
  });

  it("successful save closes the sheet, replaces to /profile, and refreshes", async () => {
    const user = userEvent.setup();
    mockSearchParamsValue = "edit=self";
    const mocks = [makeUpdateProfileMock({ input: { displayName: "Alice", bio: "hello" } })];

    render(
      <MockedProvider mocks={mocks}>
        <ProfilePageClient email="alice@example.com" initial={initial} />
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

    render(
      <MockedProvider mocks={[]}>
        <ProfilePageClient email="alice@example.com" initial={initial} />
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
        delay: 100,
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

    render(
      <MockedProvider mocks={mocks}>
        <ProfilePageClient email="alice@example.com" initial={initial} />
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
});
