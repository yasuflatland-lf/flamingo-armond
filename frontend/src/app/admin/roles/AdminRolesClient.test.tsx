// @vitest-environment jsdom

import { MockedProvider } from "@apollo/client/testing/react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AdminDeleteRoleDocument } from "@/generated/graphql";
import { AdminRolesClient, type RoleItem } from "./AdminRolesClient";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const SYSTEM_ROLES: RoleItem[] = [
  { id: "r-admin", name: "admin" },
  { id: "r-general", name: "general" },
];
const CUSTOM_ROLE: RoleItem = { id: "r-mod", name: "moderator" };

describe("AdminRolesClient", () => {
  it("wires the ListingPageShell with title, description, and New role CTA", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={[]} />
      </MockedProvider>,
    );

    expect(screen.getByRole("heading", { level: 1, name: "Roles" })).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
    expect(screen.getByText("Manage roles available to assign to users.")).toBeInTheDocument();
    // Button uses asChild + <Link>: the data-testid is forwarded to the
    // rendered <a>, so the testid handle IS the anchor element itself.
    const cta = screen.getByTestId("admin-roles-new-btn");
    expect(cta.tagName).toBe("A");
    expect(cta).toHaveAttribute("href", "/admin/roles/new");
  });

  it("renders editable rows as <Link> to the edit page", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={[CUSTOM_ROLE]} />
      </MockedProvider>,
    );

    const link = screen.getByRole("link", { name: /moderator/i });
    expect(link).toHaveAttribute("href", `/admin/roles/${CUSTOM_ROLE.id}/edit`);
  });

  it("renders system rows without an edit link and labels them as system", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={SYSTEM_ROLES} />
      </MockedProvider>,
    );

    // No anchor for system roles: the row name is plain content.
    expect(screen.queryByRole("link", { name: /admin/i })).toBeNull();
    expect(screen.queryByRole("link", { name: /general/i })).toBeNull();
    // The row carries an explicit "System role" tag.
    expect(screen.getAllByText("System role")).toHaveLength(SYSTEM_ROLES.length);
  });

  it("disables the delete button for system roles", () => {
    render(
      <MockedProvider mocks={[]}>
        <AdminRolesClient initialRoles={SYSTEM_ROLES} />
      </MockedProvider>,
    );

    expect(screen.getByTestId("admin-role-delete-btn-r-admin")).toBeDisabled();
    expect(screen.getByTestId("admin-role-delete-btn-r-general")).toBeDisabled();
  });

  it("removes a custom role from the list when delete succeeds", async () => {
    const user = userEvent.setup();

    const mocks = [
      {
        request: { query: AdminDeleteRoleDocument, variables: { id: CUSTOM_ROLE.id } },
        result: { data: { deleteRole: true } },
      },
    ];

    render(
      <MockedProvider mocks={mocks}>
        <AdminRolesClient initialRoles={[CUSTOM_ROLE]} />
      </MockedProvider>,
    );

    await user.click(screen.getByTestId(`admin-role-delete-btn-${CUSTOM_ROLE.id}`));

    await waitFor(() => {
      expect(screen.queryByTestId(`admin-role-row-${CUSTOM_ROLE.id}`)).toBeNull();
    });
  });
});
