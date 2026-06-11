import { describe, expect, it } from "vitest";
import { formatMediumDate } from "./format";

describe("formatMediumDate", () => {
  // Midday UTC keeps the calendar date stable across runner time zones.
  const iso = "2026-06-11T12:00:00Z";

  it("formats with the en-US locale (abbreviated month)", () => {
    expect(formatMediumDate(iso, "en-US")).toMatch(/Jun/);
  });

  it("honors a Japanese locale (year-first, distinct from en-US)", () => {
    const ja = formatMediumDate(iso, "ja-JP");
    // ja medium is year-first (slash or year/month/day-suffixed form depending on
    // the runtime's ICU data); en-US is month-first ("Jun 11, 2026"). Assert the
    // locale arg is honored without pinning a CJK literal (kept out of source per
    // the language policy) — the string starts with the year and differs from en.
    expect(ja).toMatch(/^2026/);
    expect(ja).not.toBe(formatMediumDate(iso, "en-US"));
  });

  it("defaults to en-US when no locale is passed", () => {
    expect(formatMediumDate(iso)).toMatch(/Jun/);
  });
});
