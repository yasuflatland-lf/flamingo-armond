// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ChangeEmailSkeleton } from "./change-email-skeleton";

describe("<ChangeEmailSkeleton>", () => {
  it("renders a busy route-shaped placeholder", () => {
    render(<ChangeEmailSkeleton />);

    expect(screen.getByTestId("change-email-skeleton")).toHaveAttribute("aria-busy", "true");
  });
});
