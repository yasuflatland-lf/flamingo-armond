// @vitest-environment node
import { describe, expect, it } from "vitest";
import { safeDecodePathSegment } from "./safe-decode-path-segment";

describe("safeDecodePathSegment", () => {
  it("(a) decodes a valid percent-encoded segment", () => {
    expect(safeDecodePathSegment("abc%26evil")).toBe("abc&evil");
  });

  it("(b) returns null for a malformed percent-escape sequence", () => {
    expect(safeDecodePathSegment("abc%XX")).toBeNull();
  });

  it("(c) returns the input unchanged for a plain alphanumeric segment", () => {
    expect(safeDecodePathSegment("abc-123")).toBe("abc-123");
  });

  it("(d) returns empty string for an empty input", () => {
    expect(safeDecodePathSegment("")).toBe("");
  });
});
