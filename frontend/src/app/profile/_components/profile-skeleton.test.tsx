// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProfileSkeleton } from "./profile-skeleton";

describe("<ProfileSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<ProfileSkeleton />);

    expect(screen.getByTestId("profile-skeleton")).toHaveAttribute("aria-busy", "true");
  });
});
