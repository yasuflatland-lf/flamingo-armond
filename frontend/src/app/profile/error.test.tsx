// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import ProfileError from "./error";

describe("<ProfileError>", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders heading and Retry button for any error", () => {
    renderWithIntl(
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

    renderWithIntl(
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

    renderWithIntl(
      <ProfileError
        error={Object.assign(new Error("Something broke"), { digest: "abc123" })}
        reset={vi.fn()}
      />,
    );

    expect(consoleSpy).toHaveBeenCalledWith(
      "[/profile error boundary]",
      expect.objectContaining({
        message: "Something broke",
        digest: "abc123",
      }),
    );

    consoleSpy.mockRestore();
  });
});
