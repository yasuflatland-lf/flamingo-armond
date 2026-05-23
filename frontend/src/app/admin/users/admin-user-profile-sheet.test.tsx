// @vitest-environment jsdom

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminUpdateUserDocument } from "@/generated/graphql";
import { AdminUserProfileSheet } from "./admin-user-profile-sheet";
import type { AdminUserListItem } from "./admin-user-role-row";

function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

function makeUser(overrides: Partial<AdminUserListItem> = {}): AdminUserListItem {
  return {
    id: "u-1",
    displayName: "Alice",
    bio: "bio text",
    avatarUrl: null,
    roles: [{ id: "r-admin", name: "admin" }],
    ...overrides,
  };
}

function renderSheet({
  user = makeUser(),
  mocks = [],
  onDismiss = vi.fn(),
  onSaved = vi.fn(),
}: {
  user?: AdminUserListItem | null;
  mocks?: React.ComponentProps<typeof MockedProvider>["mocks"];
  onDismiss?: () => void;
  onSaved?: () => void;
} = {}) {
  render(
    <MockedProvider mocks={mocks}>
      <AdminUserProfileSheet
        open
        user={user}
        loading={false}
        queryError={null}
        onDismiss={onDismiss}
        onSaved={onSaved}
      />
    </MockedProvider>,
  );
  return { onDismiss, onSaved };
}

describe("AdminUserProfileSheet", () => {
  let warnSpy: ReturnType<typeof vi.spyOn> | null = null;
  let errorSpy: ReturnType<typeof vi.spyOn> | null = null;

  afterEach(() => {
    warnSpy?.mockRestore();
    warnSpy = null;
    errorSpy?.mockRestore();
    errorSpy = null;
  });

  it("renders only display name and bio fields", () => {
    renderSheet();

    expect(screen.getByRole("heading", { name: /edit user/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/display name/i)).toHaveValue("Alice");
    expect(screen.getByLabelText(/bio/i)).toHaveValue("bio text");
    expect(screen.queryByRole("checkbox", { name: /admin/i })).not.toBeInTheDocument();
  });

  it("validates required display name before submitting", async () => {
    const user = userEvent.setup();
    renderSheet();

    await user.clear(screen.getByLabelText(/display name/i));
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    expect(screen.getByRole("alert")).toHaveTextContent("Display name is required.");
  });

  it("AdminUpdateUserSuccess closes through onSaved", async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        result: {
          data: {
            adminUpdateUser: {
              __typename: "AdminUpdateUserSuccess" as const,
              user: {
                __typename: "User" as const,
                id: "u-1",
                displayName: "Alice 2",
                bio: "bio text",
                avatarUrl: null,
                roles: [{ __typename: "Role" as const, id: "r-admin", name: "admin" }],
              },
            },
          },
        },
      },
    ];

    renderSheet({ mocks, onSaved });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("InputValidationError renders the server message", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        result: {
          data: {
            adminUpdateUser: {
              __typename: "InputValidationError" as const,
              field: "displayName",
              message: "display name is taken",
            },
          },
        },
      },
    ];

    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("display name is taken");
    });
  });

  it("FORBIDDEN transport rejection shows permission copy without logging err.message", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        error: makeCodedError("FORBIDDEN"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminUpdateUser rejected"),
      expect.objectContaining({ codes: ["FORBIDDEN"] }),
    );
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("UNAUTHENTICATED transport rejection shows session-expired copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        error: makeCodedError("UNAUTHENTICATED"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/session has expired/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminUpdateUser rejected"),
      expect.objectContaining({ codes: ["UNAUTHENTICATED"] }),
    );
  });

  it("generic transport rejection warns with empty codes and shows generic copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        error: new Error("network down"),
      },
    ];

    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/unexpected error occurred/i);
    });
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("adminUpdateUser rejected"),
      expect.objectContaining({ name: "Error", codes: [] }),
    );
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
  });

  it("unknown update payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        result: {
          data: {
            adminUpdateUser: {
              __typename: "FutureVariantClientDidNotKnowAbout",
            } as never,
          },
        },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected save payload"),
      expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
    );
  });

  it("null update payload warns and shows degraded copy", async () => {
    const user = userEvent.setup();
    const mocks = [
      {
        request: {
          query: AdminUpdateUserDocument,
          variables: {
            id: "u-1",
            input: { displayName: "Alice 2", bio: "bio text" },
          },
        },
        result: { data: { adminUpdateUser: null as never } },
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    renderSheet({ mocks });

    await user.clear(screen.getByLabelText(/display name/i));
    await user.type(screen.getByLabelText(/display name/i), "Alice 2");
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(/something went wrong/i);
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected save payload"),
      expect.objectContaining({ typename: null }),
    );
  });

  it("shows the loading indicator while the lazy query is in flight (user=null, loading=true)", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserProfileSheet
          open
          user={null}
          loading
          queryError={null}
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
        />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-user-sheet-loading")).toHaveTextContent(/loading user/i);
    expect(screen.queryByLabelText(/display name/i)).not.toBeInTheDocument();
  });

  it("renders the queryError banner without the form when the lazy query rejects", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserProfileSheet
          open
          user={null}
          loading={false}
          queryError="You do not have permission to edit this user."
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
        />
      </MockedProvider>,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/do not have permission to edit/i);
    expect(screen.queryByLabelText(/display name/i)).not.toBeInTheDocument();
  });

  it("renders 'User not found' when the lazy query settles with a null user", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminUserProfileSheet
          open
          user={null}
          loading={false}
          queryError={null}
          onDismiss={vi.fn()}
          onSaved={vi.fn()}
        />
      </MockedProvider>,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/user not found/i);
    expect(screen.queryByLabelText(/display name/i)).not.toBeInTheDocument();
  });
});
