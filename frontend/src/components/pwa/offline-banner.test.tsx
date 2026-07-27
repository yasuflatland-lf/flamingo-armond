// @vitest-environment happy-dom

import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import enMessages from "../../../messages/en.json";
import { OfflineBanner } from "./offline-banner";

const originalOnLine = Object.getOwnPropertyDescriptor(window.navigator, "onLine");

function setOnLine(value: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    value,
    configurable: true,
  });
}

afterEach(() => {
  if (originalOnLine) {
    Object.defineProperty(window.navigator, "onLine", originalOnLine);
  } else {
    Reflect.deleteProperty(window.navigator, "onLine");
  }
});

describe("<OfflineBanner>", () => {
  it("does not render when navigator reports online", () => {
    setOnLine(true);

    renderWithIntl(<OfflineBanner />);

    expect(screen.queryByTestId("offline-banner")).toBeNull();
  });

  it("renders a polite status with catalog copy when navigator reports offline", () => {
    setOnLine(false);

    renderWithIntl(<OfflineBanner />);

    const banner = screen.getByTestId("offline-banner");
    expect(banner).toHaveAttribute("role", "status");
    expect(banner).toHaveAttribute("aria-live", "polite");
    expect(banner).toHaveTextContent(enMessages.Pwa.offlineHeading);
  });
});
