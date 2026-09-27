import { describe, expect, it } from "vitest";
import { toLowerLikeGo, trimLikeGo } from "./go-text";

const GO_SPACE_CODE_POINTS = [
  0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x0085, 0x00a0, 0x1680, 0x2000, 0x2001, 0x2002,
  0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f,
  0x3000,
];

describe("trimLikeGo", () => {
  it("strips every Go unicode.IsSpace code point from both ends", () => {
    for (const codePoint of GO_SPACE_CODE_POINTS) {
      const c = String.fromCodePoint(codePoint);
      expect(trimLikeGo(`${c}a${c}`)).toBe("a");
    }
  });

  it("keeps U+FEFF, which Go TrimSpace does not strip", () => {
    expect(trimLikeGo("\uFEFF")).toBe("\uFEFF");
    expect(trimLikeGo("\uFEFFa\uFEFF")).toBe("\uFEFFa\uFEFF");
  });

  it("strips U+0085, which String.prototype.trim keeps", () => {
    expect(trimLikeGo("\u0085")).toBe("");
    expect(trimLikeGo(" \u0085a\u3000 ")).toBe("a");
  });

  it("keeps interior whitespace", () => {
    expect(trimLikeGo(" a b ")).toBe("a b");
  });

  // A backtracking edge regex takes tens of seconds here and trips the test timeout.
  it("trims a long interior whitespace run in linear time", () => {
    const s = `a${" ".repeat(300_000)}b`;
    expect(trimLikeGo(` ${s} `)).toBe(s);
  });

  it("strips nothing else in the BMP", () => {
    const stripped = [];
    for (let codePoint = 0; codePoint <= 0xffff; codePoint++) {
      if (codePoint >= 0xd800 && codePoint <= 0xdfff) continue;
      const c = String.fromCodePoint(codePoint);
      if (trimLikeGo(`${c}a${c}`) === "a") stripped.push(codePoint);
    }
    expect(stripped).toEqual(GO_SPACE_CODE_POINTS);
  });
});

describe("toLowerLikeGo", () => {
  it("maps U+0130 to plain i like Go strings.ToLower", () => {
    expect(toLowerLikeGo("ADM\u0130N")).toBe("admin");
    expect(toLowerLikeGo("\u0130")).toBe("i");
  });

  it("does not apply final-sigma context", () => {
    expect(toLowerLikeGo("\u039f\u0394\u039f\u03a3")).toBe("\u03bf\u03b4\u03bf\u03c3");
  });

  it("lowercases supplementary-plane letters per code point", () => {
    expect(toLowerLikeGo("\u{10400}")).toBe("\u{10428}");
  });

  it("lowercases ASCII", () => {
    expect(toLowerLikeGo("Admin")).toBe("admin");
  });
});
