import { describe, expect, it } from "vitest";
import { learnDayKey } from "./learn-day";

describe("learnDayKey", () => {
  it("returns the JST calendar date, not the UTC one, after 15:00 UTC", () => {
    // 2026-07-20T15:00:00Z is 2026-07-21T00:00:00+09:00 — the first instant of
    // the next JST learn day.
    expect(learnDayKey(new Date("2026-07-20T15:00:00.000Z"))).toBe("2026-07-21");
  });

  it("still returns the previous JST day one millisecond before 15:00 UTC", () => {
    // The exclusive-boundary counterpart: 14:59:59.999Z is the last instant of
    // the 2026-07-20 learn day.
    expect(learnDayKey(new Date("2026-07-20T14:59:59.999Z"))).toBe("2026-07-20");
  });

  it("agrees with the UTC date during the JST morning and afternoon", () => {
    // Between 15:00 UTC of the previous day and 15:00 UTC of this one, the
    // shifted instant stays inside the same UTC calendar date.
    expect(learnDayKey(new Date("2026-07-20T00:00:00.000Z"))).toBe("2026-07-20");
    expect(learnDayKey(new Date("2026-07-20T14:00:00.000Z"))).toBe("2026-07-20");
  });

  it("rolls the month and year over at the JST boundary", () => {
    expect(learnDayKey(new Date("2026-12-31T15:00:00.000Z"))).toBe("2027-01-01");
    expect(learnDayKey(new Date("2026-12-31T14:59:59.999Z"))).toBe("2026-12-31");
  });

  it("defaults to the current system time when called with no argument", () => {
    // Bracket the no-argument call so the assertion holds even if the clock
    // crosses the JST boundary mid-test.
    const before = learnDayKey(new Date());
    const actual = learnDayKey();
    const after = learnDayKey(new Date());
    expect([before, after]).toContain(actual);
  });
});
