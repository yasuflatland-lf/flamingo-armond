import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// The hex constants below are a hand-maintained sRGB mirror of the coral-minimal
// oklch tokens committed in `globals.css`. The contrast block proves WCAG 2.x
// AA (>= 4.5:1) for the token pairings that render text on a fill; the
// drift-guard block pins each oklch declaration verbatim so a token change in
// globals.css fails loudly and forces the hex mirror to be re-derived.

/** Parse a `#rrggbb` string into 0-255 RGB channels. */
function hexToRgb(hex: string): [number, number, number] {
  const value = hex.replace("#", "");
  const r = Number.parseInt(value.slice(0, 2), 16);
  const g = Number.parseInt(value.slice(2, 4), 16);
  const b = Number.parseInt(value.slice(4, 6), 16);
  return [r, g, b];
}

/** sRGB channel (0-255) -> linearized channel per WCAG 2.x. */
function linearize(channel: number): number {
  const c = channel / 255;
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

/** Relative luminance per WCAG 2.x. */
function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex);
  return 0.2126 * linearize(r) + 0.7152 * linearize(g) + 0.0722 * linearize(b);
}

/** WCAG 2.x contrast ratio between two hex colors (>= 1, lighter:darker). */
function contrastRatio(hex1: string, hex2: string): number {
  const l1 = relativeLuminance(hex1);
  const l2 = relativeLuminance(hex2);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

const destructive = "#c2001c";
const brandLink = "#a8353e";
const warning = "#f0b13f";
const warningForeground = "#683c00";
const destructiveForeground = "#fafafa";
const white = "#ffffff";

describe("coral-minimal token contrast", () => {
  it("destructive-foreground on destructive fill is AA", () => {
    expect(contrastRatio(destructiveForeground, destructive)).toBeGreaterThanOrEqual(4.5);
  });

  it("brand-link text on white is AA", () => {
    expect(contrastRatio(brandLink, white)).toBeGreaterThanOrEqual(4.5);
  });

  it("warning-foreground text on white is AA", () => {
    expect(contrastRatio(warningForeground, white)).toBeGreaterThanOrEqual(4.5);
  });

  it("warning-foreground on warning band is AA", () => {
    expect(contrastRatio(warningForeground, warning)).toBeGreaterThanOrEqual(4.5);
  });
});

describe("coral-minimal token declarations in globals.css", () => {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const globalsCss = readFileSync(path.resolve(here, "globals.css"), "utf8");

  const TOKEN_DECLARATIONS = [
    "--destructive: oklch(0.51 0.21 25)",
    "--ring: oklch(0.7364 0.189 18.45)",
    "--brand-link: oklch(0.5 0.15 20)",
    "--warning: oklch(0.8 0.145 78)",
    "--warning-foreground: oklch(0.4 0.1 72)",
  ] as const;

  for (const declaration of TOKEN_DECLARATIONS) {
    it(`declares ${declaration} verbatim`, () => {
      expect(globalsCss).toContain(declaration);
    });
  }
});
