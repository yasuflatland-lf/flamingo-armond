// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RootError from "./error";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("root <RootError>", () => {
  it("renders the message and a retry affordance", () => {
    render(<RootError error={new Error("boom")} reset={vi.fn()} />);

    expect(screen.getByRole("heading", { name: /couldn't load the app/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("calls reset when the retry button is clicked", () => {
    const reset = vi.fn();
    render(<RootError error={new Error("boom")} reset={reset} />);

    fireEvent.click(screen.getByRole("button", { name: /try again/i }));

    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("logs with the [error] scope prefix and redacts the raw message", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const error = Object.assign(new Error("secret-user-content"), { digest: "abc123" });

    render(<RootError error={error} reset={vi.fn()} />);

    expect(spy).toHaveBeenCalledWith("[error]", { name: "Error", digest: "abc123" });
  });
});
