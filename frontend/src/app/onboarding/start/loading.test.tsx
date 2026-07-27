// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loading from "./loading";

describe("/onboarding/start <Loading>", () => {
  it("renders the onboarding-start skeleton", () => {
    render(<Loading />);

    expect(screen.getByTestId("onboarding-start-skeleton")).toBeInTheDocument();
  });
});
