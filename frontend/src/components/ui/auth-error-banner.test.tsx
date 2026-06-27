// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AuthErrorBanner } from "./auth-error-banner";

describe("<AuthErrorBanner>", () => {
  it("renders the resolved message and a /login link with the given label and testId", () => {
    render(
      <AuthErrorBanner
        testId="some-auth-error"
        message="Your session has expired."
        signInLabel="Sign in again"
      />,
    );

    const banner = screen.getByTestId("some-auth-error");
    expect(banner).toHaveTextContent("Your session has expired.");

    const link = screen.getByRole("link", { name: "Sign in again" });
    expect(link).toHaveAttribute("href", "/login");
  });

  it("forwards an optional className to the rendered banner", () => {
    render(
      <AuthErrorBanner
        testId="some-auth-error"
        message="You do not have permission."
        signInLabel="Sign in again"
        className="mb-4"
      />,
    );

    expect(screen.getByTestId("some-auth-error")).toHaveClass("mb-4");
  });
});
