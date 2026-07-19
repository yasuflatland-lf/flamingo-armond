// @vitest-environment happy-dom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";

describe("Button variants", () => {
  it("destructiveGhost renders low-emphasis ghost-red (text-destructive on transparent, no filled bg)", () => {
    render(<Button variant="destructiveGhost">Delete</Button>);
    const button = screen.getByRole("button", { name: "Delete" });
    expect(button).toHaveClass("bg-transparent", "text-destructive", "hover:bg-destructive/10");
    // The low-emphasis trigger must NOT be the filled crimson commit variant.
    expect(button).not.toHaveClass("bg-destructive");
  });

  it("destructiveOutline adds a visible border to the ghost-red trigger without filling it", () => {
    render(<Button variant="destructiveOutline">Delete my account</Button>);
    const button = screen.getByRole("button", { name: "Delete my account" });
    expect(button).toHaveClass(
      "border",
      "border-destructive/45",
      "bg-transparent",
      "text-destructive",
      "hover:bg-destructive/10",
    );
    // Still a trigger, not the commit: the border buys affordance, not emphasis.
    expect(button).not.toHaveClass("bg-destructive");
  });

  it("link variant uses the coral link token, not the neutral primary", () => {
    render(<Button variant="link">Learn more</Button>);
    const button = screen.getByRole("button", { name: "Learn more" });
    expect(button).toHaveClass("text-brand-link", "underline-offset-4", "hover:underline");
    expect(button).not.toHaveClass("text-primary");
  });

  it("destructive (filled commit) keeps the crimson fill", () => {
    render(<Button variant="destructive">Confirm delete</Button>);
    const button = screen.getByRole("button", { name: "Confirm delete" });
    expect(button).toHaveClass("bg-destructive", "text-destructive-foreground");
  });
});
