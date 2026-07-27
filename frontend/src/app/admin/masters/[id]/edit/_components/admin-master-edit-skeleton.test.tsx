// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AdminMasterEditSkeleton } from "./admin-master-edit-skeleton";

describe("<AdminMasterEditSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<AdminMasterEditSkeleton />);

    expect(screen.getByTestId("admin-master-edit-skeleton")).toHaveAttribute("aria-busy", "true");
  });
});
