// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OnboardingSkeleton } from "./onboarding-skeleton";

describe("<OnboardingSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<OnboardingSkeleton />);

    expect(screen.getByTestId("onboarding-skeleton")).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("main")).toBeInTheDocument();
  });
});
