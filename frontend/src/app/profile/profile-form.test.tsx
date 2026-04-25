// @vitest-environment jsdom
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";
import { UpdateProfileDocument } from "@/generated/graphql";
import { ProfileForm } from "./profile-form";

// Stub next/navigation so ProfileForm can render outside Next.js
vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));

// Helper to build a fully typed MockedResponse for UpdateProfile.
// __typename fields are required so Apollo's InMemoryCache can normalise the result.
function makeMutationMock(
  variables: { input: { displayName: string; bio: string | null } },
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
              bio: variables.input.bio,
              avatarUrl: null,
            },
          },
        },
      };
    },
  };
}

describe("<ProfileForm>", () => {
  it("renders initial values from props", () => {
    render(
      <MockedProvider mocks={[]}>
        <ProfileForm initial={{ displayName: "Alice", bio: "Hello world" }} />
      </MockedProvider>,
    );

    expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Hello world")).toBeInTheDocument();
  });

  it("blocks submit when displayName is empty (form validation prevents mutation call)", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    const mocks = [makeMutationMock({ input: { displayName: "", bio: null } }, mutationCalled)];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "", bio: "" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    // Zod schema rejects empty displayName — mutation must not be called
    await waitFor(() => {
      expect(screen.getByText(/display name is required/i)).toBeInTheDocument();
    });
    expect(mutationCalled).not.toHaveBeenCalled();
  });

  it("submits with trimmed displayName", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    // The Zod schema trims displayName before it reaches the mutation,
    // so "  Alice  " becomes "Alice" in the variables.
    const mocks = [
      makeMutationMock({ input: { displayName: "Alice", bio: null } }, mutationCalled),
    ];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "", bio: "" }} />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.clear(displayNameInput);
    await user.type(displayNameInput, "  Alice  ");

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("bio empty string sends input.bio === null to the mutation", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    // bio="" in initial props triggers "bio || undefined" → bio is undefined in form state.
    // The form converts undefined bio to null when building mutation variables.
    const mocks = [
      makeMutationMock({ input: { displayName: "Alice", bio: null } }, mutationCalled),
    ];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "Alice", bio: "" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("renders error message when mutation returns a GraphQL error", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: null } },
        },
        result: {
          errors: [
            new GraphQLError("displayName must be 1-50 characters", {
              extensions: { code: "BAD_USER_INPUT" },
            }),
          ],
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks} defaultOptions={{ mutate: { errorPolicy: "all" } }}>
        <ProfileForm initial={{ displayName: "", bio: "" }} />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.type(displayNameInput, "Alice");

    const saveButton = screen.getByRole("button", { name: /save/i });
    await user.click(saveButton);

    expect(await screen.findByText("displayName must be 1-50 characters")).toBeInTheDocument();
  });
});
