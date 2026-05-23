// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

// Static regression guard: read the sonner.tsx source and assert that the
// actionButton className contains bg-brand-primary. This pins the brand
// accent decision without requiring a runtime render of Toaster.
describe("Toaster (sonner.tsx) brand accent regression guard", () => {
  it("actionButton className contains bg-brand-primary", () => {
    const source = readFileSync(resolve(__dirname, "./sonner.tsx"), "utf-8");
    expect(source).toMatch(/bg-brand-primary/);
  });
});
