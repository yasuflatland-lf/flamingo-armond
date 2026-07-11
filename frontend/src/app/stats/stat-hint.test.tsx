// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { StatHint } from "./stat-hint";

const props = {
  title: "Retention",
  body: "Recalled by due date.",
  ariaLabel: "About Retention",
};

describe("StatHint", () => {
  it("exposes an accessible help trigger", () => {
    render(<StatHint {...props} />);
    expect(screen.getByRole("button", { name: "About Retention" })).toBeInTheDocument();
  });

  it("hides the body until the trigger is activated", () => {
    render(<StatHint {...props} />);
    expect(screen.queryByText("Recalled by due date.")).not.toBeInTheDocument();
  });

  it("reveals the title and body on click", async () => {
    const user = userEvent.setup();
    render(<StatHint {...props} />);
    await user.click(screen.getByRole("button", { name: "About Retention" }));
    expect(screen.getByText("Recalled by due date.")).toBeInTheDocument();
    expect(screen.getByText("Retention")).toBeInTheDocument();
  });

  it("closes on Escape", async () => {
    const user = userEvent.setup();
    render(<StatHint {...props} />);
    await user.click(screen.getByRole("button", { name: "About Retention" }));
    expect(screen.getByText("Recalled by due date.")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByText("Recalled by due date.")).not.toBeInTheDocument();
  });
});
