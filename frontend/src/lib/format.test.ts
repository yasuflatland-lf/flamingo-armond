import { describe, expect, it } from "vitest";
import { formatDateTime, formatMediumDate } from "./format";

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

describe("formatDateTime", () => {
  // Midday UTC keeps the calendar date stable across runner time zones.
  const iso = "2026-06-15T12:00:00.000Z";

  it("renders date and time for the en-US locale", () => {
    const out = formatDateTime(iso, "en-US");
    // Year is timezone-stable for a midday-UTC instant; a time component
    // (colon-separated H:MM) must also render.
    expect(out).toMatch(/2026/);
    expect(out).toMatch(/\d{1,2}:\d{2}/);
  });

  it("honors a Japanese locale (year-first, distinct from en-US)", () => {
    const ja = formatDateTime(iso, "ja-JP");
    expect(ja).toMatch(/^2026/);
    expect(ja).not.toBe(formatDateTime(iso, "en-US"));
  });

  it('accepts the app\'s bare locale tags ("ja"), as passed by useLocale()', () => {
    // The admin row passes next-intl's bare "ja"/"en" tags, not "ja-JP".
    const ja = formatDateTime(iso, "ja");
    expect(ja).toMatch(/^2026/);
    expect(ja).not.toBe(formatDateTime(iso, "en"));
  });

  it("defaults to en-US when no locale is passed", () => {
    expect(formatDateTime(iso)).toMatch(/\d{1,2}:\d{2}/);
  });
});
