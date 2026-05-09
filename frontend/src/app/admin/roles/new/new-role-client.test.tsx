// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GraphQLError } from "graphql";
import { describe, expect, it, vi } from "vitest";

import { AdminCreateRoleDocument } from "@/generated/graphql";
import { NewRoleClient } from "./new-role-client";

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
  it("renders the form heading and cancel link", () => {
    render(
      <MockedProvider mocks={[]}>
        <NewRoleClient />
      </MockedProvider>,
    );
    expect(screen.getByRole("heading", { name: /new role/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /cancel/i })).toHaveAttribute("href", "/admin/roles");
  });

  it("submits a normalized name and navigates on success", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mutationCalled = vi.fn();
    const mocks = [
      {
        // Zod transforms input to trim+lowercase before submit, so the
        // mutation sees the canonical form regardless of the user typing
        // mixed-case or padded text.
        request: { query: AdminCreateRoleDocument, variables: { name: "moderator" } },
        result: () => {
          mutationCalled();
          return {
            data: {
              createRole: { __typename: "Role" as const, id: "r-new", name: "moderator" },
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

  it("surfaces a BAD_USER_INPUT field error inline", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminCreateRoleDocument, variables: { name: "admin" } },
        result: {
          errors: [
            new GraphQLError("role name already exists", {
              extensions: { code: "BAD_USER_INPUT", field: "name" },
            }),
          ],
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks} defaultOptions={{ mutate: { errorPolicy: "all" } }}>
        <NewRoleClient />
      </MockedProvider>,
    );

    await user.type(screen.getByRole("textbox"), "admin");
    await user.click(screen.getByRole("button", { name: /create/i }));

    const errorEl = await screen.findByText("role name already exists");
    expect(errorEl).toBeInTheDocument();
    expect(errorEl.className).toMatch(/text-destructive/);
  });
});
