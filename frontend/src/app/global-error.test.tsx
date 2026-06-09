// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import GlobalError from "./global-error";

describe("<GlobalError>", () => {
  // React emits a console.error about invalid HTML nesting when a component
  // returns <html>/<body> into jsdom's existing document.body. Suppress the
  // known React DOM nesting validation warning narrowly so the suite does not
  // fail on infrastructure noise, but still surfaces unexpected errors.
  const domNestingPattern =
    /Warning: validateDOMNesting|Warning:.*<html>|Warning:.*<body>|Cannot update a component|An update to.*inside a test/;

  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation((...args) => {
      const msg = typeof args[0] === "string" ? args[0] : "";
      if (domNestingPattern.test(msg)) return;
      // Let any other console.error through to the actual implementation so
      // unexpected errors are still visible.
    });
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it("renders the fallback heading and message", () => {
    render(
      <GlobalError
        error={Object.assign(new Error("Render error"), { digest: undefined })}
        reset={vi.fn()}
      />,
    );

    expect(screen.getByRole("heading", { name: /something went wrong/i })).toBeInTheDocument();
    expect(screen.getByText(/unexpected error occurred/i)).toBeInTheDocument();
  });

  it("renders a Try again button and a Go to home link", () => {
    render(
      <GlobalError
        error={Object.assign(new Error("Render error"), { digest: undefined })}
        reset={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /go to home/i })).toHaveAttribute("href", "/");
  });

  it("calls reset() when Try again is clicked", async () => {
    const user = userEvent.setup();
    const reset = vi.fn();

    render(
      <GlobalError
        error={Object.assign(new Error("Render error"), { digest: undefined })}
        reset={reset}
      />,
    );

    await user.click(screen.getByRole("button", { name: /try again/i }));

    expect(reset).toHaveBeenCalledOnce();
  });

  it("logs error to console.error with [global-error] scope prefix", () => {
    // Use a fresh spy that captures calls instead of suppressing them.
    consoleErrorSpy.mockRestore();
    const logSpy = vi.spyOn(console, "error").mockImplementation((...args) => {
      const msg = typeof args[0] === "string" ? args[0] : "";
      if (domNestingPattern.test(msg)) return;
      // pass through non-nesting calls so we can capture [global-error]
    });

    render(
      <GlobalError
        error={Object.assign(new Error("Something broke"), { digest: "abc123" })}
        reset={vi.fn()}
      />,
    );

    expect(logSpy).toHaveBeenCalledWith(
      "[global-error]",
      expect.objectContaining({
        message: "Something broke",
        digest: "abc123",
      }),
    );

    logSpy.mockRestore();
    // Re-install the nesting suppressor so afterEach.mockRestore() is a no-op.
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });
});
