// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/cardgroups/[id]/edit <Loading>", () => {
  it("renders the cardgroup-edit skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("cardgroup-edit-skeleton")).toBeInTheDocument();
  });
});
