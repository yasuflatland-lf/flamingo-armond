// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProfileError from "./error";

const mockReplace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace }),
}));

describe("<ProfileError>", () => {
  beforeEach(() => {
    mockReplace.mockClear();
  });

  it("calls router.replace('/login') when error contains UNAUTHENTICATED", () => {
    render(
      <ProfileError
        error={Object.assign(new Error("GraphQL errors: UNAUTHENTICATED"), { digest: undefined })}
        reset={vi.fn()}
      />,
    );

    expect(mockReplace).toHaveBeenCalledWith("/login");
  });

  it("renders heading and Retry button when error is not UNAUTHENTICATED", () => {
    render(
      <ProfileError
        error={Object.assign(new Error("Network error"), { digest: undefined })}
        reset={vi.fn()}
      />,
    );

    expect(screen.getByText("Couldn't load your profile")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("calls reset() when Retry button is clicked", async () => {
    const user = userEvent.setup();
    const reset = vi.fn();

    render(
      <ProfileError
        error={Object.assign(new Error("Network error"), { digest: undefined })}
        reset={reset}
      />,
    );

    await user.click(screen.getByRole("button", { name: /retry/i }));

    expect(reset).toHaveBeenCalledOnce();
  });

  it("logs error with digest to console.error", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    render(
      <ProfileError
        error={Object.assign(new Error("Something broke"), { digest: "abc123" })}
        reset={vi.fn()}
      />,
    );

    expect(consoleSpy).toHaveBeenCalledWith(
      "[/profile error boundary]",
      expect.objectContaining({ digest: "abc123" }),
    );

    consoleSpy.mockRestore();
  });

  it("only calls router.replace once even when the effect re-runs", () => {
    const error = Object.assign(new Error("GraphQL errors: UNAUTHENTICATED"), {
      digest: undefined,
    });

    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const { rerender } = render(<ProfileError error={error} reset={vi.fn()} />);

    // Re-render with the same error object to simulate an effect re-run.
    rerender(<ProfileError error={error} reset={vi.fn()} />);

    expect(mockReplace).toHaveBeenCalledOnce();

    consoleSpy.mockRestore();
  });
});
