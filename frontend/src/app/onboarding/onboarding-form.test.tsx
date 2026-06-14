// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
import { renderWithIntl } from "@/test/render-with-intl";
import { OnboardingForm } from "./onboarding-form";

// Stub next/navigation so OnboardingForm can render outside Next.js.
const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeUpdateProfileMock(
  variables: { input: { displayName: string; bio?: string | null } },
  onCalled?: () => void,
) {
  return {
    request: {
      query: UpdateProfileDocument,
      variables,
    },
    result: () => {
      onCalled?.();
      return {
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
      };
    },
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("<OnboardingForm>", () => {
  it("empty submit is blocked — validation error displayed", async () => {
    const user = userEvent.setup();

    renderWithIntl(
      <MockedProvider mocks={[]}>
        <OnboardingForm />
      </MockedProvider>,
    );

    // Blur the empty displayName field to trigger the onBlur validator.
    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText(/display name is required/i)).toBeInTheDocument();
    });
  });

  it("valid submit calls UpdateProfile mutation with the typed input", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    const mocks = [makeUpdateProfileMock({ input: { displayName: "Alice" } }, mutationCalled)];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("onCompleted triggers router.push('/onboarding/start')", async () => {
    const user = userEvent.setup();

    const mocks = [makeUpdateProfileMock({ input: { displayName: "Alice" } })];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/onboarding/start");
    });
  });

  it("InputValidationError variant surfaces field error in the form", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice" } },
        },
        result: {
          data: {
            updateProfile: {
              __typename: "InputValidationError" as const,
              field: "displayName",
              message: "displayName must be 1-50 characters",
            },
          },
        },
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    const errorEl = await screen.findByText("displayName must be 1-50 characters");
    expect(errorEl).toBeInTheDocument();
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("keeps formState.isSubmitSuccessful=false after a rejecting submit (regression: inner catch+throw pattern)", async () => {
    const user = userEvent.setup();

    // A network-level rejection causes the mutation promise to reject. The inner
    // .catch + throw in onSubmit keeps formState.isSubmitSuccessful=false because
    // the re-thrown error propagates through form.handleSubmit(), which marks the
    // submit as unsuccessful. The outer .catch at the JSX call site swallows the
    // re-throw to avoid an unhandled browser promise rejection.
    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice" } },
        },
        error: new Error("network down"),
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Wait for the mutation rejection to propagate and settle.
    await waitFor(() => {
      const sentinel = screen.getByTestId("is-submit-successful");
      // isSubmitSuccessful must remain "false" — a "true" here means onSubmit swallowed
      // the rejection and TanStack Form incorrectly treated the submit as successful.
      expect(sentinel).toHaveAttribute("data-value", "false");
    });
  });

  it("network error surfaces as a banner-level alert (not a field error)", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice" } },
        },
        error: new Error("Network request failed"),
      },
    ];

    renderWithIntl(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      const banner = screen.getByRole("alert");
      expect(banner).toBeInTheDocument();
      expect(banner.textContent ?? "").toMatch(/could not reach the server|network/i);
    });
  });
});
