// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AdminQueryErrorBanner } from "./admin-query-error-banner";

const COPY = {
  viewForbidden: "You do not have permission to view this page.",
  sessionExpired: "Your session has expired.",
  signInAgain: "Please sign in again.",
  retry: "Retry",
};

describe("<AdminQueryErrorBanner>", () => {
  it("renders nothing when kind is null", () => {
    const { container } = render(
      <AdminQueryErrorBanner
        kind={null}
        onRetry={vi.fn()}
        copy={COPY}
        testId="admin-query-error"
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the forbidden copy without a Retry button for the forbidden kind", () => {
    render(
      <AdminQueryErrorBanner
        kind={{ kind: "forbidden" }}
        onRetry={vi.fn()}
        copy={COPY}
        testId="admin-query-error"
      />,
    );

    const banner = screen.getByTestId("admin-query-error");
    expect(banner).toHaveTextContent(COPY.viewForbidden);
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  it("renders the session-expired copy with a /login link for the unauthenticated kind", () => {
    render(
      <AdminQueryErrorBanner
        kind={{ kind: "unauthenticated" }}
        onRetry={vi.fn()}
        copy={COPY}
        testId="admin-query-error"
      />,
    );

    const banner = screen.getByTestId("admin-query-error");
    expect(banner).toHaveTextContent(COPY.sessionExpired);

    const link = screen.getByRole("link", { name: COPY.signInAgain });
    expect(link).toHaveAttribute("href", "/login");
    expect(screen.queryByRole("button", { name: /retry/i })).not.toBeInTheDocument();
  });

  it("renders the banner message and a Retry button for the banner kind", () => {
    render(
      <AdminQueryErrorBanner
        kind={{ kind: "banner", message: "Something went wrong" }}
        onRetry={vi.fn()}
        copy={COPY}
        testId="admin-query-error"
      />,
    );

    const banner = screen.getByTestId("admin-query-error");
    expect(banner).toHaveTextContent("Something went wrong");
    expect(screen.getByRole("button", { name: COPY.retry })).toBeInTheDocument();
  });

  it("invokes onRetry with no arguments when the Retry button is clicked", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <AdminQueryErrorBanner
        kind={{ kind: "banner", message: "boom" }}
        onRetry={onRetry}
        copy={COPY}
        testId="admin-query-error"
      />,
    );

    await user.click(screen.getByRole("button", { name: COPY.retry }));

    expect(onRetry).toHaveBeenCalledOnce();
    // The click event must not leak through as a refetch variables argument.
    expect(onRetry).toHaveBeenCalledWith();
  });

  it("applies the testId and an optional className to the rendered banner", () => {
    render(
      <AdminQueryErrorBanner
        kind={{ kind: "forbidden" }}
        onRetry={vi.fn()}
        copy={COPY}
        testId="admin-users-query-error"
        className="mb-4"
      />,
    );

    const banner = screen.getByTestId("admin-users-query-error");
    expect(banner).toHaveClass("mb-4");
  });
});
