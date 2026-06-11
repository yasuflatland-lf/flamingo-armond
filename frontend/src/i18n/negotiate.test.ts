import { describe, expect, it } from "vitest";
import { negotiateLocale, resolveLocale } from "./negotiate";

describe("negotiateLocale", () => {
  it("returns ja when Japanese is the top preference", () => {
    expect(negotiateLocale("ja,en;q=0.8")).toBe("ja");
    expect(negotiateLocale("ja-JP,ja;q=0.9,en;q=0.8")).toBe("ja");
  });

  it("returns en when English outranks Japanese", () => {
    expect(negotiateLocale("en-US,en;q=0.9,ja;q=0.5")).toBe("en");
  });

  it("demotes tags with malformed q-weights (NaN coerced to 0, then dropped)", () => {
    expect(negotiateLocale("ja;q=abc,en;q=0.5")).toBe("en");
    expect(negotiateLocale("ja;q=,en;q=0.9")).toBe("en");
  });

  it("drops tags explicitly rejected with q=0 (RFC 7231)", () => {
    expect(negotiateLocale("ja;q=0,en;q=0.1")).toBe("en");
    // ja rejected and no other supported tag -> default.
    expect(negotiateLocale("ja;q=0")).toBe("en");
  });

  it("breaks q-ties by source order (stable sort)", () => {
    expect(negotiateLocale("ja;q=0.5,en;q=0.5")).toBe("ja");
    expect(negotiateLocale("en;q=0.5,ja;q=0.5")).toBe("en");
    // Implicit q=1 on both -> first listed wins.
    expect(negotiateLocale("ja,en")).toBe("ja");
  });

  it("falls back to the default for unsupported or missing headers", () => {
    expect(negotiateLocale("fr-FR,de;q=0.7")).toBe("en");
    expect(negotiateLocale(null)).toBe("en");
    expect(negotiateLocale("")).toBe("en");
  });
});

describe("resolveLocale", () => {
  it("prefers a valid cookie over Accept-Language", () => {
    expect(resolveLocale("ja", "en-US,en;q=0.9")).toBe("ja");
    expect(resolveLocale("en", "ja")).toBe("en");
  });

  it("falls through to Accept-Language when the cookie is absent or unsupported", () => {
    expect(resolveLocale(undefined, "ja")).toBe("ja");
    expect(resolveLocale(null, "ja")).toBe("ja");
    expect(resolveLocale("fr", "ja")).toBe("ja"); // unsupported cookie ignored
    expect(resolveLocale("", "en")).toBe("en");
  });

  it("falls back to the default when neither cookie nor header matches", () => {
    expect(resolveLocale(undefined, undefined)).toBe("en");
    expect(resolveLocale("xx", "fr")).toBe("en");
  });
});
