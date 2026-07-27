// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OnboardingStartSkeleton } from "./onboarding-start-skeleton";

describe("<OnboardingStartSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<OnboardingStartSkeleton />);

    expect(screen.getByTestId("onboarding-start-skeleton")).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("main")).toBeInTheDocument();
  });
});
