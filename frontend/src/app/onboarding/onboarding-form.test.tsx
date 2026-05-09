// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
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
            __typename: "UpdateProfilePayload" as const,
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

    render(
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

    const mocks = [
      makeUpdateProfileMock({ input: { displayName: "Alice" } }, mutationCalled),
    ];

    render(
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

  it("onCompleted triggers router.push('/cardgroups/new?welcome=1')", async () => {
    const user = userEvent.setup();

    const mocks = [makeUpdateProfileMock({ input: { displayName: "Alice" } })];

    render(
      <MockedProvider mocks={mocks}>
        <OnboardingForm />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    await user.type(displayNameInput, "Alice");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/cardgroups/new?welcome=1");
    });
  });

  it("backend BAD_USER_INPUT field error surfaces in the form", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice" } },
        },
        result: {
          errors: [
            new GraphQLError("displayName must be 1-50 characters", {
              extensions: { code: "BAD_USER_INPUT", field: "displayName" },
            }),
          ],
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks} defaultOptions={{ mutate: { errorPolicy: "all" } }}>
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
});
