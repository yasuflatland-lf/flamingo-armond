// @vitest-environment happy-dom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UpdateLearnDisplayModeDocument, UpdateProfileDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { DisplayModeSection } from "./display-mode-section";
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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
        />
      </MockedProvider>,
    );

    const editButton = screen.getByRole("button", { name: /edit profile/i });
    // Icon-only: the visible label was removed; the accessible name comes from aria-label.
    expect(editButton).not.toHaveTextContent(/edit profile/i);

    await user.click(editButton);

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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
        />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { name: /settings/i })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /language/i })).toBeInTheDocument();
  });

  it("renders the new-card-ratio section for admins", () => {
    mockSearchParamsValue = "";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={true}
        />
      </MockedProvider>,
    );

    // The section (and its slider) are wired into the settings section only for
    // admins — proving the render-gating prop is threaded from the consumer.
    expect(screen.getByRole("slider", { name: /new card ratio/i })).toBeInTheDocument();
    expect(screen.getByTestId("new-card-ratio-slider")).toBeInTheDocument();
  });

  it("does not render the new-card-ratio section for non-admins", () => {
    mockSearchParamsValue = "";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
        />
      </MockedProvider>,
    );

    expect(screen.queryByRole("slider", { name: /new card ratio/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId("new-card-ratio-slider")).not.toBeInTheDocument();
  });

  it("opens the sheet when ?edit=self (singleton sentinel) is present in the query", () => {
    mockSearchParamsValue = "edit=self";

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <ProfilePageClient
          email="alice@example.com"
          initial={initial}
          displayMode="FLIP_TO_REVEAL"
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
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
          newCardRatio={{ __typename: "NewCardRatio", numerator: 4, denominator: 5 }}
          isAdmin={false}
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

describe("<DisplayModeSection> error handling and no-op guard", () => {
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  it("reverts the toggle and shows an inline error when the mutation rejects", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: UpdateLearnDisplayModeDocument,
          variables: { mode: "ALWAYS_VISIBLE" },
        },
        result: {
          errors: [new GraphQLError("not signed in", { extensions: { code: "UNAUTHENTICATED" } })],
        },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <DisplayModeSection initialMode="FLIP_TO_REVEAL" />
      </MockedProvider>,
    );

    const flipOption = screen.getByTestId("display-mode-flip-to-reveal");
    const alwaysVisibleOption = screen.getByTestId("display-mode-always-visible");

    // FLIP_TO_REVEAL is active on mount.
    expect(flipOption).toHaveAttribute("aria-pressed", "true");
    expect(alwaysVisibleOption).toHaveAttribute("aria-pressed", "false");

    await user.click(alwaysVisibleOption);

    // After the rejection the optimistic selection rolls back to FLIP_TO_REVEAL.
    await waitFor(() => {
      expect(flipOption).toHaveAttribute("aria-pressed", "true");
      expect(alwaysVisibleOption).toHaveAttribute("aria-pressed", "false");
    });

    // An inline error is surfaced to the user.
    const alert = await screen.findByRole("alert");
    expect(alert).toBe(screen.getByTestId("display-mode-error"));
    expect(alert).toHaveTextContent(/.+/);

    // The rejection is logged for operator triage with the lifted codes only.
    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[profile] updateLearnDisplayMode rejected",
      expect.objectContaining({ codes: expect.arrayContaining(["UNAUTHENTICATED"]) }),
    );
  });

  it("reverts the toggle and shows the permission-denied error on FORBIDDEN", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: UpdateLearnDisplayModeDocument,
          variables: { mode: "ALWAYS_VISIBLE" },
        },
        result: {
          errors: [new GraphQLError("not allowed", { extensions: { code: "FORBIDDEN" } })],
        },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <DisplayModeSection initialMode="FLIP_TO_REVEAL" />
      </MockedProvider>,
    );

    const flipOption = screen.getByTestId("display-mode-flip-to-reveal");
    const alwaysVisibleOption = screen.getByTestId("display-mode-always-visible");

    await user.click(alwaysVisibleOption);

    // After the FORBIDDEN rejection the optimistic selection rolls back.
    await waitFor(() => {
      expect(flipOption).toHaveAttribute("aria-pressed", "true");
      expect(alwaysVisibleOption).toHaveAttribute("aria-pressed", "false");
    });

    // The FORBIDDEN branch surfaces the permission-denied copy, distinct from the
    // session-expired copy used for UNAUTHENTICATED.
    const alert = await screen.findByRole("alert");
    expect(alert).toBe(screen.getByTestId("display-mode-error"));
    expect(alert).toHaveTextContent(/permission/i);

    expect(consoleWarnSpy).toHaveBeenCalledWith(
      "[profile] updateLearnDisplayMode rejected",
      expect.objectContaining({ codes: expect.arrayContaining(["FORBIDDEN"]) }),
    );
  });

  it("does not fire the mutation when the already-active option is clicked", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();
    // Provide a mock for the active mode; if the no-op guard fails this mock is
    // consumed and the spy records a call, failing the assertion below.
    const mocks = [makeUpdateLearnDisplayModeMock("FLIP_TO_REVEAL", mutationCalled)];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <DisplayModeSection initialMode="FLIP_TO_REVEAL" />
      </MockedProvider>,
    );

    const flipOption = screen.getByTestId("display-mode-flip-to-reveal");
    expect(flipOption).toHaveAttribute("aria-pressed", "true");

    await user.click(flipOption);

    // Give any pending effect a tick to settle, then assert the mutation never fired.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mutationCalled).not.toHaveBeenCalled();
  });

  it("does not carry optimisticResponse in the updateLearnDisplayMode mutation", () => {
    // Static assertion: updateLearnDisplayMode can return UNAUTHENTICATED (a typed
    // GraphQL error). Apollo v3 does not reliably roll back optimistic writes on
    // typed GraphQL errors — only on network errors. So no `optimisticResponse`
    // must appear in the mutate call; the component rolls back manually in .catch.
    // See .claude/rules/pagination.md § "Drop `optimisticResponse` for mutations
    // that can fail with typed GraphQL errors".
    //
    // Strategy: slice the source between the `updateMode({` mutate call and the
    // `} catch (` that follows it, and assert no `optimisticResponse` key appears.
    const source = DisplayModeSection.toString();

    const mutateStart = source.indexOf("updateMode({");
    expect(mutateStart).toBeGreaterThan(-1);

    const catchIdx = source.indexOf("} catch (", mutateStart);
    expect(catchIdx).toBeGreaterThan(-1);

    const mutateBlock = source.slice(mutateStart, catchIdx);
    expect(mutateBlock).not.toContain("optimisticResponse");
  });
});
