// @vitest-environment jsdom
import { fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { InAppBrowserGuard } from "./in-app-browser-guard";

const REAL_UA = navigator.userAgent;

function setUserAgent(ua: string) {
  Object.defineProperty(navigator, "userAgent", { value: ua, configurable: true });
}

const LINE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Safari/604.1 Line/14.5.0";
const INSTAGRAM_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 333.0.0.0";
const WEBVIEW_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/UQ1A.240205.004; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/125.0.0.0 Mobile Safari/537.36";
const SAFARI_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";

afterEach(() => {
  setUserAgent(REAL_UA);
  vi.restoreAllMocks();
});

describe("<InAppBrowserGuard>", () => {
  it("renders nothing in a standalone browser", () => {
    setUserAgent(SAFARI_UA);
    renderWithIntl(<InAppBrowserGuard />);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows a blocking modal with the external-browser action inside LINE", () => {
    setUserAgent(LINE_UA);
    renderWithIntl(<InAppBrowserGuard />);

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /open in browser/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /copy page link/i })).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /cancel|continue|close|keep/i }),
    ).not.toBeInTheDocument();
  });

  it("shows a blocking modal with a copy-link fallback inside other in-app browsers", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    setUserAgent(INSTAGRAM_UA);
    renderWithIntl(<InAppBrowserGuard />);

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    const copyButton = screen.getByRole("button", { name: /copy page link/i });
    expect(screen.queryByRole("button", { name: /open in browser/i })).not.toBeInTheDocument();

    fireEvent.click(copyButton);

    expect(writeText).toHaveBeenCalledWith(window.location.href);
    expect(await screen.findByText(/paste into safari or chrome/i)).toBeInTheDocument();
  });

  it("shows the low-key inline banner (not a modal) for a generic Android WebView", () => {
    setUserAgent(WEBVIEW_UA);
    renderWithIntl(<InAppBrowserGuard />);

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copy page link/i })).toBeInTheDocument();
  });
});
