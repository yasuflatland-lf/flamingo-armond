// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/profile <Loading>", () => {
  it("renders the profile skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("profile-skeleton")).toBeInTheDocument();
  });
});
