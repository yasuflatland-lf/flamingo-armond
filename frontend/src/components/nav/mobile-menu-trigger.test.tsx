// @vitest-environment jsdom
import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithIntl } from "@/test/render-with-intl";
import { Sheet } from "@/components/ui/sheet";
import { MobileMenuTrigger } from "./mobile-menu-trigger";

describe("<MobileMenuTrigger>", () => {
  it("S-M1: button has aria-label 'Open menu'", () => {
    renderWithIntl(
      <Sheet>
        <MobileMenuTrigger />
      </Sheet>,
    );
    expect(screen.getByRole("button", { name: "Open menu" })).toBeInTheDocument();
  });

  it("S-M2: icon svg is hidden from assistive technology", () => {
    const { container } = renderWithIntl(
      <Sheet>
        <MobileMenuTrigger />
      </Sheet>,
    );
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg).toHaveAttribute("aria-hidden", "true");
  });
});
