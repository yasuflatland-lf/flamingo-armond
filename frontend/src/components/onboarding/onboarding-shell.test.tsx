// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OnboardingShell } from "./onboarding-shell";

describe("OnboardingShell", () => {
  it("renders the heading as the single h1 and the children", () => {
    render(
      <OnboardingShell heading="Welcome">
        <p>step content</p>
      </OnboardingShell>,
    );

    const headings = screen.getAllByRole("heading", { level: 1 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent("Welcome");
    expect(screen.getByText("step content")).toBeInTheDocument();
  });

  it("renders the subline when provided", () => {
    render(
      <OnboardingShell heading="Welcome" subline="Set up your name.">
        <span />
      </OnboardingShell>,
    );

    expect(screen.getByText("Set up your name.")).toBeInTheDocument();
  });

  it("omits the subline paragraph when none is given", () => {
    render(
      <OnboardingShell heading="Welcome">
        <span />
      </OnboardingShell>,
    );

    // Heading only — no supporting paragraph element rendered.
    expect(screen.queryByText("Set up your name.")).not.toBeInTheDocument();
  });

  it("forwards headingId to the h1 so callers keep a stable anchor", () => {
    render(
      <OnboardingShell heading="Welcome" headingId="welcome-heading">
        <span />
      </OnboardingShell>,
    );

    expect(screen.getByRole("heading", { level: 1 })).toHaveAttribute("id", "welcome-heading");
  });

  it("renders the decorative FlamingoMark", () => {
    const { container } = render(
      <OnboardingShell heading="Welcome">
        <span />
      </OnboardingShell>,
    );

    expect(container.querySelector("svg")).not.toBeNull();
  });
});
