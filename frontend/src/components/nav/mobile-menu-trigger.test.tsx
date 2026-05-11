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
});
