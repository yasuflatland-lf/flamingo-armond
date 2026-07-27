// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatsSkeleton } from "./stats-skeleton";

describe("<StatsSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<StatsSkeleton />);

    expect(screen.getByTestId("stats-skeleton")).toHaveAttribute("aria-busy", "true");
  });
});
