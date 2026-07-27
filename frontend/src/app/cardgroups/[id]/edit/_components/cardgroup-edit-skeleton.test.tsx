// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardgroupEditSkeleton } from "./cardgroup-edit-skeleton";

describe("<CardgroupEditSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<CardgroupEditSkeleton />);

    expect(screen.getByTestId("cardgroup-edit-skeleton")).toHaveAttribute("aria-busy", "true");
  });
});
