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
function makeMutationMock(
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

describe("<ProfileForm>", () => {
  it("renders defaults", () => {
    render(
      <MockedProvider mocks={[]}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    expect(screen.getByDisplayValue("Alice")).toBeInTheDocument();
    expect(screen.getByDisplayValue("hi")).toBeInTheDocument();
  });

  it("displayName empty triggers Zod error before submit", async () => {
    const user = userEvent.setup();

    render(
      <MockedProvider mocks={[]}>
        <ProfileForm initial={{ displayName: "Alice", bio: "" }} />
      </MockedProvider>,
    );

    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.clear(displayNameInput);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText(/display name is required/i)).toBeInTheDocument();
    });
  });

  it("displayName 51 graphemes triggers Zod error", async () => {
    const user = userEvent.setup();

    render(
      <MockedProvider mocks={[]}>
        <ProfileForm initial={{ displayName: "", bio: "" }} />
      </MockedProvider>,
    );

    // Use a 51-grapheme string (simple ASCII for stability in CI)
    const longName = "a".repeat(51);
    const displayNameInput = screen.getByLabelText(/display name/i);
    await user.click(displayNameInput);
    // paste is faster and avoids per-keystroke debounce issues
    await user.paste(longName);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText(/display name must be 50 characters or fewer/i)).toBeInTheDocument();
    });
  });

  it("bio over 500 triggers Zod error", async () => {
    const user = userEvent.setup();

    render(
      <MockedProvider mocks={[]}>
        <ProfileForm initial={{ displayName: "Alice", bio: "" }} />
      </MockedProvider>,
    );

    const longBio = "b".repeat(501);
    const bioTextarea = screen.getByLabelText(/^bio$/i);
    await user.click(bioTextarea);
    await user.paste(longBio);
    await user.tab();

    await waitFor(() => {
      expect(screen.getByText(/bio must be 500 characters or fewer/i)).toBeInTheDocument();
    });
  });

  it("Clear bio button sets bio to empty and includes it in submit", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    // bio="" in mutation variables means explicit clear
    const mocks = [makeMutationMock({ input: { displayName: "Alice", bio: "" } }, mutationCalled)];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    // "Clear bio" button appears because bio is "hi" (non-empty)
    const clearBtn = screen.getByRole("button", { name: /clear bio/i });
    await user.click(clearBtn);

    // After clearing, the Clear bio button should disappear
    expect(screen.queryByRole("button", { name: /clear bio/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("submit posts UpdateProfile mutation with displayName and bio", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    const mocks = [
      makeMutationMock({ input: { displayName: "Alice", bio: "hello" } }, mutationCalled),
    ];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hello" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });

  it("backend BAD_USER_INPUT is shown under the corresponding field", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: "hi" } },
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
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    const errorEl = await screen.findByText("displayName must be 1-50 characters");
    expect(errorEl).toBeInTheDocument();
    expect(errorEl.className).toMatch(/text-destructive/);
  });

  it("backend INTERNAL error is shown as a generic banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: "hi" } },
        },
        result: {
          errors: [
            new GraphQLError("Something went wrong on the server", {
              extensions: { code: "INTERNAL" },
            }),
          ],
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks} defaultOptions={{ mutate: { errorPolicy: "all" } }}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByText("Something went wrong on the server")).toBeInTheDocument();
    });
  });

  it("backend UNAUTHENTICATED error is shown with sign-in prompt", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: "hi" } },
        },
        result: {
          errors: [
            new GraphQLError("Unauthenticated", {
              extensions: { code: "UNAUTHENTICATED" },
            }),
          ],
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks} defaultOptions={{ mutate: { errorPolicy: "all" } }}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(screen.getByText("Your session expired. Please sign in again.")).toBeInTheDocument();
    });
  });

  it("network error is shown as connectivity banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice", bio: "hi" } },
        },
        error: new Error("network down"),
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "Alice", bio: "hi" }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Could not reach the server. Check your connection and try again."),
      ).toBeInTheDocument();
    });
  });

  it("bio untouched undefined sends mutation without bio variable", async () => {
    const user = userEvent.setup();
    const mutationCalled = vi.fn();

    // bio is undefined (not sent) when defaultValues.bio is undefined and untouched
    const mocks = [
      {
        request: {
          query: UpdateProfileDocument,
          variables: { input: { displayName: "Alice" } },
        },
        result: () => {
          mutationCalled();
          return {
            data: {
              updateProfile: {
                __typename: "UpdateProfilePayload" as const,
                user: {
                  __typename: "User" as const,
                  id: "user-1",
                  displayName: "Alice",
                  bio: null,
                  avatarUrl: null,
                },
              },
            },
          };
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <ProfileForm initial={{ displayName: "Alice", bio: undefined as unknown as string }} />
      </MockedProvider>,
    );

    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
  });
});
