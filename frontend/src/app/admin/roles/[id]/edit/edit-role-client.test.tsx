// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AdminUpdateRoleDocument } from "@/generated/graphql";
import { EditRoleClient, type RoleForEdit } from "./edit-role-client";

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

const CUSTOM_ROLE: RoleForEdit = { id: "r-mod", name: "moderator" };
const ADMIN_ROLE: RoleForEdit = { id: "r-admin", name: "admin" };

describe("EditRoleClient", () => {
  it("renders heading + cancel link", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={CUSTOM_ROLE} />
      </MockedProvider>,
    );
    expect(screen.getByRole("heading", { name: /edit role/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /cancel/i })).toHaveAttribute("href", "/admin/roles");
  });

  it("submits update mutation and navigates on success", async () => {
    const user = userEvent.setup();
    mockPush.mockClear();
    mockRefresh.mockClear();

    const mutationCalled = vi.fn();
    const mocks = [
      {
        // Zod transforms input to trim+lowercase before submit, so the
        // mutation sees "reviewer" regardless of mixed-case typed input.
        request: {
          query: AdminUpdateRoleDocument,
          variables: { id: CUSTOM_ROLE.id, name: "reviewer" },
        },
        result: () => {
          mutationCalled();
          return {
            data: {
              updateRole: { __typename: "Role" as const, id: CUSTOM_ROLE.id, name: "reviewer" },
            },
          };
        },
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <EditRoleClient role={CUSTOM_ROLE} />
      </MockedProvider>,
    );

    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "Reviewer");
    await user.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => {
      expect(mutationCalled).toHaveBeenCalledOnce();
    });
    expect(mockPush).toHaveBeenCalledWith("/admin/roles");
    expect(mockRefresh).toHaveBeenCalledOnce();
  });

  it("renders the system-role banner and disables submit when role is admin", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={ADMIN_ROLE} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-role-edit-system-banner")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("renders the system-role banner for the general role", () => {
    render(
      <MockedProvider mocks={[]}>
        <EditRoleClient role={{ id: "r-general", name: "general" }} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-role-edit-system-banner")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });
});
