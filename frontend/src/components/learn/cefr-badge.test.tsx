// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CefrLevel } from "@/generated/graphql";
import { CefrBadge } from "./cefr-badge";

const LEVEL_TO_BAND = [
  { level: "A1", bg: "bg-cefr-a", fg: "text-cefr-a-foreground" },
  { level: "A2", bg: "bg-cefr-a", fg: "text-cefr-a-foreground" },
  { level: "B1", bg: "bg-cefr-b", fg: "text-cefr-b-foreground" },
  { level: "B2", bg: "bg-cefr-b", fg: "text-cefr-b-foreground" },
  { level: "C1", bg: "bg-cefr-c", fg: "text-cefr-c-foreground" },
  { level: "C2", bg: "bg-cefr-c", fg: "text-cefr-c-foreground" },
] as const;

describe("<CefrBadge>", () => {
  for (const { level, bg, fg } of LEVEL_TO_BAND) {
    it(`maps ${level} to the correct band with matching foreground`, () => {
      render(<CefrBadge level={level} />);
      const el = screen.getByLabelText(`CEFR level ${level}`);
      expect(el).toHaveClass(bg, fg);
    });
  }

  it("renders nothing when the level is null", () => {
    render(<CefrBadge level={null} />);
    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
  });

  it("renders nothing when the level is undefined (defensive == null guard)", () => {
    render(<CefrBadge level={undefined as unknown as null} />);
    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
  });

  it("suppresses the badge and warns for an out-of-union runtime level", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(<CefrBadge level={"D1" as unknown as CefrLevel} />);

    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("D1"));

    warn.mockRestore();
  });

  it("exposes the exact accessible name for the level", () => {
    render(<CefrBadge level="B2" />);
    const el = screen.getByLabelText("CEFR level B2");
    expect(el).toHaveAccessibleName("CEFR level B2");
  });

  it("carries role=img so the aria-label is reliably announced", () => {
    render(<CefrBadge level="B2" />);
    const el = screen.getByLabelText("CEFR level B2");
    expect(el).toHaveAttribute("role", "img");
  });

  it("is non-interactive: no tabindex and no button role", () => {
    render(<CefrBadge level="A1" />);
    const el = screen.getByLabelText("CEFR level A1");
    expect(el).not.toHaveAttribute("tabindex");
    expect(el).not.toHaveAttribute("role", "button");
    expect(screen.queryByRole("button")).toBeNull();
  });
});

// --- Static contrast guard (pure unit test) ---------------------------------
//
// These hex values mirror the oklch CEFR tokens committed in
// `frontend/src/app/globals.css` and MUST be kept in sync with them. The test
// proves WCAG 2.x contrast for (a) text-on-its-own-band-fill (readability) and
// (b) text-on-white (the white card surface acceptance criterion).

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

const WHITE = "#ffffff";

const BANDS = [
  { band: "a", bg: "#caf0d1", fg: "#00631f" },
  { band: "b", bg: "#ffdfb1", fg: "#784100" },
  { band: "c", bg: "#ffd4d8", fg: "#921238" },
] as const;

describe("CEFR token contrast", () => {
  for (const { band, bg, fg } of BANDS) {
    it(`band ${band}: text-on-band-fill contrast >= 4.5`, () => {
      expect(contrastRatio(fg, bg)).toBeGreaterThanOrEqual(4.5);
    });

    it(`band ${band}: text-on-white contrast >= 4.5`, () => {
      expect(contrastRatio(fg, WHITE)).toBeGreaterThanOrEqual(4.5);
    });
  }
});

// --- Token drift guard (static source) --------------------------------------
//
// The hex constants above are a hand-maintained mirror of the `--cefr-*`
// oklch tokens in globals.css. Pin each token's exact committed declaration
// so that a token change in globals.css fails this test loudly — forcing the
// hex constants here to be re-derived and re-verified for contrast.

describe("CEFR token declarations in globals.css", () => {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const globalsCss = readFileSync(path.resolve(here, "../../app/globals.css"), "utf8");

  const TOKEN_DECLARATIONS = [
    "--cefr-a: oklch(92% 0.058 150)",
    "--cefr-a-foreground: oklch(43% 0.14 150)",
    "--cefr-b: oklch(92% 0.07 75)",
    "--cefr-b-foreground: oklch(43% 0.13 75)",
    "--cefr-c: oklch(91% 0.052 12)",
    "--cefr-c-foreground: oklch(43% 0.16 12)",
  ] as const;

  for (const declaration of TOKEN_DECLARATIONS) {
    it(`declares ${declaration} verbatim`, () => {
      expect(globalsCss).toContain(declaration);
    });
  }
});

afterEach(() => {
  vi.restoreAllMocks();
});
