// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MemoryBar } from "./memory-bar";

describe("<MemoryBar>", () => {
  it("renders an element with role progressbar", () => {
    render(<MemoryBar stability={10} />);

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });

  it("sets aria-valuenow to 12 for stability 2.5", () => {
    render(<MemoryBar stability={2.5} />);

    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "12");
  });

  it("sets aria-valuenow to 100 for stability 21", () => {
    render(<MemoryBar stability={21} />);

    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "100");
  });

  it("sets aria-valuenow to 0 for stability 0", () => {
    render(<MemoryBar stability={0} />);

    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "0");
  });
});
