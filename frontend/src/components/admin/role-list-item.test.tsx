// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { RoleListItem } from "./role-list-item";

describe("RoleListItem", () => {
  it("fires onDelete with the role id when the delete button inside the real SwipeableRow is clicked", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();

    renderWithIntl(
      <ul>
        <RoleListItem
          id="r-mod"
          name="moderator"
          isSystem={false}
          busy={false}
          onEdit={vi.fn()}
          onDelete={onDelete}
        />
      </ul>,
    );

    expect(screen.getByTestId("swipeable-row")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /delete moderator/i }));

    expect(onDelete).toHaveBeenCalledWith("r-mod");
  });
});
