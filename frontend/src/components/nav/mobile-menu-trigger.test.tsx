// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Sheet } from "@/components/ui/sheet";
import { MobileMenuTrigger } from "./mobile-menu-trigger";

describe("<MobileMenuTrigger>", () => {
  it("S-M1: button has aria-label 'Open menu'", () => {
    render(
      <Sheet>
        <MobileMenuTrigger />
      </Sheet>,
    );
    expect(screen.getByRole("button", { name: "Open menu" })).toBeInTheDocument();
  });

  it("S-M2: renders a Settings (lucide) icon as svg", () => {
    const { container } = render(
      <Sheet>
        <MobileMenuTrigger />
      </Sheet>,
    );
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    // lucide-react sets class names like "lucide lucide-settings"
    expect(svg?.getAttribute("class") ?? "").toMatch(/lucide/);
  });
});
