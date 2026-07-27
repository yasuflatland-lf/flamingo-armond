// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/admin/masters/[id]/edit <Loading>", () => {
  it("renders the admin-master-edit skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("admin-master-edit-skeleton")).toBeInTheDocument();
  });
});
