// @vitest-environment happy-dom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, render, screen } from "@testing-library/react";
import { toast } from "sonner";
import { describe, expect, it } from "vitest";
import { Toaster } from "./sonner";

// Regression guard for the brand-accent decision: the default toast surface is the
// brand coral, driven through sonner's --normal-* CSS custom properties (utility
// classes lose to sonner's own selectors, so the colour MUST flow through the vars).
describe("Toaster (sonner.tsx) brand accent regression guard", () => {
  it("drives the default toast surface off the brand palette (source guard)", () => {
    const source = readFileSync(resolve(__dirname, "./sonner.tsx"), "utf-8");
    expect(source).toMatch(/--normal-bg["']?\s*:\s*["']?var\(--brand-primary/);
    expect(source).toMatch(/--normal-text["']?\s*:\s*["']?var\(--brand-primary-foreground/);
  });

  it("applies the brand-coral surface vars to the rendered toast", async () => {
    render(<Toaster />);
    act(() => {
      toast("Test", { action: { label: "Undo", onClick: () => {} } });
    });
    const surface = (await screen.findByText("Test")).closest(
      "[data-sonner-toast]",
    ) as HTMLElement | null;
    expect(surface).not.toBeNull();
    expect(surface?.style.getPropertyValue("--normal-bg")).toContain("brand-primary");
    expect(surface?.style.getPropertyValue("--normal-text")).toContain("brand-primary-foreground");
  });
});
