// @vitest-environment jsdom
import { fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { InAppBrowserNotice } from "./in-app-browser-notice";

const REAL_UA = navigator.userAgent;

function setUserAgent(ua: string) {
  Object.defineProperty(navigator, "userAgent", { value: ua, configurable: true });
}

const LINE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Safari/604.1 Line/14.5.0";
const INSTAGRAM_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 333.0.0.0";
const SAFARI_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";

afterEach(() => {
  setUserAgent(REAL_UA);
  vi.restoreAllMocks();
});

describe("<InAppBrowserNotice>", () => {
  it("renders nothing in a standalone browser", () => {
    setUserAgent(SAFARI_UA);
    renderWithIntl(<InAppBrowserNotice />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("warns and offers the external-browser action inside LINE", () => {
    setUserAgent(LINE_UA);
    renderWithIntl(<InAppBrowserNotice />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    // LINE gets the one-tap escape hatch, not the copy-link fallback.
    expect(screen.getByRole("button", { name: /open in browser/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /copy page link/i })).not.toBeInTheDocument();
  });

  it("warns and offers a copy-link fallback inside other in-app browsers", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    setUserAgent(INSTAGRAM_UA);
    renderWithIntl(<InAppBrowserNotice />);

    const copyButton = screen.getByRole("button", { name: /copy page link/i });
    expect(copyButton).toBeInTheDocument();

    fireEvent.click(copyButton);

    expect(writeText).toHaveBeenCalledWith(window.location.href);
    // After a successful copy the label confirms the action.
    expect(await screen.findByText(/paste into safari or chrome/i)).toBeInTheDocument();
  });
});
