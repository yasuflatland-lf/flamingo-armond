// @vitest-environment node
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { safeDecodePathSegment } from "./safe-decode-path-segment";

describe("safeDecodePathSegment", () => {
  it("(a) decodes a valid percent-encoded segment", () => {
    expect(safeDecodePathSegment("abc%26evil")).toBe("abc&evil");
  });

  it("(b) returns null for a malformed percent-escape sequence", () => {
    expect(safeDecodePathSegment("abc%XX")).toBeNull();
  });

  it("inverts encodeURIComponent for any well-formed string (property)", () => {
    fc.assert(
      fc.property(fc.string({ unit: "binary" }), (s) => {
        expect(safeDecodePathSegment(encodeURIComponent(s))).toBe(s);
      }),
    );
  });

  it("returns a segment without '%' unchanged, never throwing (property)", () => {
    fc.assert(
      fc.property(fc.string({ unit: "binary" }), (s) => {
        const plain = s.replaceAll("%", "");
        expect(safeDecodePathSegment(plain)).toBe(plain);
      }),
    );
  });
});
