// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/profile/change-email <Loading>", () => {
  it("renders the change-email skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("change-email-skeleton")).toBeInTheDocument();
  });
});
