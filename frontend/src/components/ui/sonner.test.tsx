// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, render, screen } from "@testing-library/react";
import { toast } from "sonner";
import { describe, expect, it } from "vitest";
import { Toaster } from "./sonner";

// Static regression guard: read the sonner.tsx source and assert that the
// actionButton className contains bg-brand-primary. This pins the brand
// accent decision without requiring a runtime render of Toaster.
describe("Toaster (sonner.tsx) brand accent regression guard", () => {
  it("actionButton className contains bg-brand-primary", () => {
    const source = readFileSync(resolve(__dirname, "./sonner.tsx"), "utf-8");
    expect(source).toMatch(/bg-brand-primary/);
  });

  it("renders action button with brand-primary accent at runtime", async () => {
    render(<Toaster />);
    act(() => {
      toast("Test", { action: { label: "Undo", onClick: () => {} } });
    });
    const undoBtn = await screen.findByRole("button", { name: /undo/i });
    expect(undoBtn.className).toMatch(/bg-brand-primary/);
  });
});
