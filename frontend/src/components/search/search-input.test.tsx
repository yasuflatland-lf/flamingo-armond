// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SearchInput } from "./search-input";

describe("<SearchInput>", () => {
  it("renders a search input carrying the design-system focus-ring token", () => {
    render(
      <SearchInput
        placeholder="Search decks..."
        aria-label="Search decks"
        value=""
        onChange={() => {}}
      />,
    );
    const input = screen.getByRole("searchbox", { name: "Search decks" });
    expect(input).toHaveAttribute("type", "search");
    expect(input).toHaveAttribute("placeholder", "Search decks...");
    expect(input).toHaveClass("focus-visible:ring-ring", "focus-visible:ring-offset-2");
  });

  it("fires onChange on keystroke", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<SearchInput aria-label="Search" value="" onChange={onChange} />);
    await user.type(screen.getByRole("searchbox", { name: "Search" }), "a");
    expect(onChange).toHaveBeenCalled();
  });

  it("forwards data-testid to the underlying input element", () => {
    render(
      <SearchInput aria-label="Search" value="" onChange={() => {}} data-testid="my-search" />,
    );
    expect(screen.getByTestId("my-search")).toBe(screen.getByRole("searchbox", { name: "Search" }));
  });

  it("renders no leading icon by default", () => {
    render(
      <SearchInput aria-label="Search" value="" onChange={() => {}} data-testid="my-search" />,
    );
    const input = screen.getByTestId("my-search");
    // No icon variant: the input's parent is whatever the caller wraps it in,
    // not an internal relative-positioned icon container.
    expect(input.parentElement?.querySelector("svg")).not.toBeInTheDocument();
  });

  it("renders a leading search icon when icon is set", () => {
    render(
      <SearchInput icon aria-label="Search" value="" onChange={() => {}} data-testid="my-search" />,
    );
    const input = screen.getByTestId("my-search");
    expect(input.parentElement?.querySelector("svg")).toBeInTheDocument();
    expect(input).toHaveClass("pl-8");
  });
});
