// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/stats <Loading>", () => {
  it("renders the stats skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("stats-skeleton")).toBeInTheDocument();
  });
});
