// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/onboarding <Loading>", () => {
  it("renders the onboarding skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("onboarding-skeleton")).toBeInTheDocument();
  });
});
