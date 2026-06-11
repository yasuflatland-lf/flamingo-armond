// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  type AdminUserListItem,
  type AdminUserRole,
  AdminUserRow,
} from "@/app/admin/users/admin-user-row";
import { renderWithIntl } from "@/test/render-with-intl";

vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    width,
    height,
    ...rest
  }: {
    src: string;
    alt: string;
    width: number;
    height: number;
    [key: string]: unknown;
  }) => (
    // biome-ignore lint/performance/noImgElement: deliberate next/image stub for tests
    <img src={src} alt={alt} width={width} height={height} {...rest} />
  ),
}));

const GENERAL_ROLE: AdminUserRole = { id: "role-general", name: "general" };

function makeUser(overrides: Partial<AdminUserListItem> = {}): AdminUserListItem {
  return {
    id: "user-1",
    version: 0,
    displayName: "User 1",
    bio: null,
    avatarUrl: null,
    roles: [GENERAL_ROLE],
    ...overrides,
  };
}

describe("AdminUserRow", () => {
  it("renders user identity without inline role toggles", () => {
    renderWithIntl(
      <ul>
        <AdminUserRow user={makeUser()} onEdit={vi.fn()} />
      </ul>,
    );

    expect(screen.getByText("User 1")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /general/i })).not.toBeInTheDocument();
  });

  it("invokes onEdit from the row edit affordance", async () => {
    const user = userEvent.setup({ delay: null });
    const onEdit = vi.fn();

    renderWithIntl(
      <ul>
        <AdminUserRow user={makeUser()} onEdit={onEdit} />
      </ul>,
    );

    await user.click(screen.getByRole("button", { name: /edit user 1/i }));

    expect(onEdit).toHaveBeenCalledWith("user-1");
  });
});
