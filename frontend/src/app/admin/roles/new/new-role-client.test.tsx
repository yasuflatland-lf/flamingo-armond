// @vitest-environment jsdom

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AdminCreateRoleDocument } from "@/generated/graphql";
import { NewRoleClient } from "./new-role-client";

/**
 * Build a CombinedGraphQLErrors carrying a single extension code. Mirrors the
 * runtime shape Apollo Client v4 surfaces to useMutation's catch path — this
 * is the shape liftGraphQLCodes narrows via CombinedGraphQLErrors.is.
 */
function makeCodedError(code: string): CombinedGraphQLErrors {
  return new CombinedGraphQLErrors({
    errors: [{ message: "transport", extensions: { code } }],
  });
}

const mockPush = vi.fn();
const mockRefresh = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

describe("NewRoleClient", () => {
  // Capture console.warn for MockedProvider leak detection — see
  // .claude/rules/pagination.md § "Capture console.warn for MockedProvider
  // leaks, then assert in teardown".
  let warnSpy: ReturnType<typeof vi.spyOn> | null = null;

  afterEach(() => {
    warnSpy?.mockRestore();
    warnSpy = null;
    mockPush.mockClear();
    mockRefresh.mockClear();
  });

  it("renders the form heading and cancel link", () => {
    render(
      <MockedProvider mocks={[]}>
        <NewRoleClient />
      </MockedProvider>,
    );
    expect(screen.getByRole("heading", { name: /new role/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /cancel/i })).toHaveAttribute("href", "/admin/roles");
  });

  it("CreateRoleSuccess — submits a normalized name and navigates", async () => {
    const user = userEvent.setup();

    const mutationCalled = vi.fn();
    const mocks = [
      {
        // Zod transforms input to trim+lowercase before submit, so the
        // mutation sees the canonical form regardless of mixed-case typed input.
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => {
          mutationCalled();
          return {
            data: {
              createRole: {
                __typename: "CreateRoleSuccess" as const,
                role: { __typename: "Role" as const, id: "r-new", name: "moderator" },
              },
            },
          };
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "  Moderator  ");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockPush).toHaveBeenCalledWith("/admin/roles");
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("InputValidationError (name field) — shows banner inline, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "admin" } },
        result: () => ({
          data: {
            createRole: {
              __typename: "InputValidationError" as const,
              field: "name",
              message: "role name already exists",
            },
          },
        }),
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "admin");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-new-validation-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-role-new-validation-error")).toHaveTextContent(
      "role name already exists",
    );
    // No navigation: the error variant is data, not a success.
    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
  });

  it("unauthenticated transport rejection — shows login link banner, no navigation", async () => {
    const user = userEvent.setup();

    // Apollo runtime shape: CombinedGraphQLErrors with extensions.code.
    // liftGraphQLCodes narrows via CombinedGraphQLErrors.is.
    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        error: makeCodedError("UNAUTHENTICATED"),
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "moderator");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-new-auth-error")).toBeInTheDocument();
    });
    const banner = screen.getByTestId("admin-role-new-auth-error");
    expect(banner).toHaveTextContent(/session has expired/i);
    const signIn = within(banner).getByRole("link", { name: /sign in again/i });
    expect(signIn).toHaveAttribute("href", "/login");

    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
    // Auth branch returns early — generic warn must NOT fire.
    expect(warnSpy).not.toHaveBeenCalled();
  });

  it("forbidden transport rejection — shows login link banner, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        error: makeCodedError("FORBIDDEN"),
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "moderator");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("admin-role-new-auth-error")).toBeInTheDocument();
    });
    expect(screen.getByTestId("admin-role-new-auth-error")).toHaveTextContent(
      /do not have permission/i,
    );

    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(warnSpy).not.toHaveBeenCalled();
  });

  it("generic transport rejection — warns and shows no auth/validation banner", async () => {
    const user = userEvent.setup();

    const networkError = new Error("network down");
    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        error: networkError,
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "moderator");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());

    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();
    expect(screen.queryByTestId("admin-role-new-auth-error")).not.toBeInTheDocument();
    expect(screen.queryByTestId("admin-role-new-validation-error")).not.toBeInTheDocument();
    expect(screen.queryByTestId("admin-role-new-unexpected-payload-error")).not.toBeInTheDocument();

    // Warn payload MUST NOT include err.message — backend messages may echo user input.
    expect(warnSpy).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ message: expect.anything() }),
    );
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("createRole rejected"),
      expect.objectContaining({ name: "Error", codes: [] }),
    );
  });

  it("unexpected __typename — warns and shows degraded banner, no navigation", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => ({
          data: {
            createRole: {
              __typename: "FutureVariantClientDidNotKnowAbout",
            } as never,
          },
        }),
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "moderator");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());

    expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toBeInTheDocument();
    expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toHaveTextContent(
      /something went wrong/i,
    );
    expect(screen.queryByTestId("admin-role-new-validation-error")).not.toBeInTheDocument();

    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();

    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected createRole payload"),
      expect.objectContaining({ typename: "FutureVariantClientDidNotKnowAbout" }),
    );
  });

  it("null createRole payload (partial-response null bubble) — warns and shows banner", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => ({
          data: { createRole: null as never },
        }),
      },
    ];

    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(
      <MockedProvider mocks={mocks}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "moderator");
    await user.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => expect(warnSpy).toHaveBeenCalled());

    expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toBeInTheDocument();
    expect(screen.getByTestId("admin-role-new-unexpected-payload-error")).toHaveTextContent(
      /something went wrong/i,
    );

    expect(mockPush).not.toHaveBeenCalled();
    expect(mockRefresh).not.toHaveBeenCalled();

    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("unexpected createRole payload"),
      expect.objectContaining({ typename: null }),
    );
  });
});
