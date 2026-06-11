// @vitest-environment jsdom

import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { type AdminUserListItem, AdminUserRow } from "./admin-user-row";

vi.mock("next/image", () => ({
  default: (props: { src: string; alt: string; width: number; height: number }) => (
    // biome-ignore lint/performance/noImgElement: jsdom-friendly stand-in for next/image
    // biome-ignore lint/a11y/useAltText: alt is forwarded from props
    <img {...props} />
  ),
}));

function makeUser(overrides: Partial<AdminUserListItem> = {}): AdminUserListItem {
  return {
    id: "u-1",
    version: 41,
    displayName: "Alice",
    bio: "bio text",
    avatarUrl: null,
    roles: [{ id: "r-admin", name: "admin" }],
    ...overrides,
  };
}

describe("AdminUserRow", () => {
  it("renders identity and invokes onEdit without inline role checkboxes", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();

    renderWithIntl(
      <ul>
        <AdminUserRow user={makeUser()} onEdit={onEdit} />
      </ul>,
    );

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("bio text")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /edit alice/i }));

    expect(onEdit).toHaveBeenCalledWith("u-1");
  });
});
