// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CefrLevel } from "@/generated/base-types";
import { CefrBadge } from "./cefr-badge";

describe("<CefrBadge>", () => {
  it("maps each level to its band color", () => {
    render(<CefrBadge level={CefrLevel.A1} />);
    expect(screen.getByLabelText("CEFR level A1")).toHaveClass("bg-cefr-a");
  });

  it("maps B-band levels to the b color", () => {
    render(<CefrBadge level={CefrLevel.B1} />);
    expect(screen.getByLabelText("CEFR level B1")).toHaveClass("bg-cefr-b");
  });

  it("maps C-band levels to the c color", () => {
    render(<CefrBadge level={CefrLevel.C1} />);
    expect(screen.getByLabelText("CEFR level C1")).toHaveClass("bg-cefr-c");
  });

  it("renders nothing when the level is null", () => {
    render(<CefrBadge level={null} />);
    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
  });

  it("renders nothing when the level is undefined (defensive == null guard)", () => {
    render(<CefrBadge level={undefined as unknown as null} />);
    expect(screen.queryByLabelText(/CEFR level/)).toBeNull();
  });

  it("exposes the exact accessible label for the level", () => {
    render(<CefrBadge level={CefrLevel.B2} />);
    expect(screen.getByLabelText("CEFR level B2")).toBeInTheDocument();
    // The label text is exactly "CEFR level B2" — no more, no less.
    expect(screen.queryByLabelText("CEFR level B2 ")).toBeNull();
  });

  it("is non-interactive: no tabindex and no button role", () => {
    render(<CefrBadge level={CefrLevel.A1} />);
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
  { band: "a", bg: "#d9f3dd", fg: "#00682a" },
  { band: "b", bg: "#ffebce", fg: "#7b4800" },
  { band: "c", bg: "#ffe2e5", fg: "#96233f" },
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
