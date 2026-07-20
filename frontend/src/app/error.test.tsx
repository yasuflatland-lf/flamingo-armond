// @vitest-environment happy-dom
import { fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import enMessages from "../../messages/en.json";
import jaMessages from "../../messages/ja.json";
import RootError from "./error";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("root <RootError>", () => {
  it("renders the message and a retry affordance", () => {
    renderWithIntl(<RootError error={new Error("boom")} reset={vi.fn()} />);

    expect(screen.getByRole("heading", { name: enMessages.RootError.heading })).toBeInTheDocument();
    expect(screen.getByText(enMessages.RootError.message)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: enMessages.RootError.retry })).toBeInTheDocument();
  });

  it("renders all three strings from the catalog in the active locale", () => {
    renderWithIntl(<RootError error={new Error("boom")} reset={vi.fn()} />, {
      locale: "ja",
      messages: jaMessages,
    });

    expect(screen.getByRole("heading", { name: jaMessages.RootError.heading })).toBeInTheDocument();
    expect(screen.getByText(jaMessages.RootError.message)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: jaMessages.RootError.retry })).toBeInTheDocument();
  });

  it("calls reset when the retry button is clicked", () => {
    const reset = vi.fn();
    renderWithIntl(<RootError error={new Error("boom")} reset={reset} />);

    fireEvent.click(screen.getByRole("button", { name: enMessages.RootError.retry }));

    expect(reset).toHaveBeenCalledTimes(1);
  });

  it("logs with the [error] scope prefix and redacts the raw message", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    const error = Object.assign(new Error("secret-user-content"), { digest: "abc123" });

    renderWithIntl(<RootError error={error} reset={vi.fn()} />);

    expect(spy).toHaveBeenCalledWith("[error]", { name: "Error", digest: "abc123" });
    // Assert the raw message is absent explicitly, so a future switch to a
    // partial matcher (e.g. objectContaining) cannot silently let it through.
    expect(JSON.stringify(spy.mock.calls[0]?.[1])).not.toContain("secret-user-content");
    expect(spy).toHaveBeenCalledTimes(1);
  });
});
