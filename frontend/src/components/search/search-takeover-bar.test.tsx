// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { SearchTakeoverBar } from "./search-takeover-bar";

afterEach(() => vi.restoreAllMocks());

function baseProps() {
  return {
    open: true,
    value: "",
    onChange: vi.fn(),
    onClear: vi.fn(),
    onClose: vi.fn(),
    placeholder: "Filter cardgroups...",
    ariaLabel: "Filter cardgroups",
  };
}

describe("<SearchTakeoverBar>", () => {
  it("renders nothing when closed", () => {
    renderWithIntl(<SearchTakeoverBar {...baseProps()} open={false} />);
    expect(screen.queryByTestId("search-takeover")).not.toBeInTheDocument();
  });

  it("renders a search landmark and autofocuses the input when open", () => {
    renderWithIntl(<SearchTakeoverBar {...baseProps()} />);
    expect(screen.getByRole("search")).toBeInTheDocument();
    const input = screen.getByTestId("search-takeover-input");
    expect(input).toHaveFocus();
    expect(input).toHaveAttribute("aria-label", "Filter cardgroups");
  });

  it("calls onChange on keystroke", async () => {
    const user = userEvent.setup();
    const props = baseProps();
    renderWithIntl(<SearchTakeoverBar {...props} />);
    await user.type(screen.getByTestId("search-takeover-input"), "a");
    expect(props.onChange).toHaveBeenCalled();
  });

  it("hides the clear button when value is empty", () => {
    renderWithIntl(<SearchTakeoverBar {...baseProps()} value="" />);
    expect(screen.queryByRole("button", { name: "Clear search" })).not.toBeInTheDocument();
  });

  it("shows and fires the clear button when value is non-empty", async () => {
    const user = userEvent.setup();
    const props = baseProps();
    renderWithIntl(<SearchTakeoverBar {...props} value="deck" />);
    const clearBtn = screen.getByRole("button", { name: "Clear search" });
    await user.click(clearBtn);
    expect(props.onClear).toHaveBeenCalledTimes(1);
  });

  it("calls onClose from the back button", async () => {
    const user = userEvent.setup();
    const props = baseProps();
    renderWithIntl(<SearchTakeoverBar {...props} />);
    await user.click(screen.getByRole("button", { name: "Close search" }));
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose on Escape", async () => {
    const user = userEvent.setup();
    const props = baseProps();
    renderWithIntl(<SearchTakeoverBar {...props} />);
    await user.keyboard("{Escape}");
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });
});
