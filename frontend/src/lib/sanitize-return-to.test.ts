// @vitest-environment node
import { describe, expect, it } from "vitest";
import { sanitizeReturnTo } from "./sanitize-return-to";

function mulberry32(seed: number): () => number {
  let s = seed | 0;
  return () => {
    s = (s + 0x6d2b79f5) | 0;
    let t = Math.imul(s ^ (s >>> 15), 1 | s);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const ALPHABET = [
  "/",
  "/",
  "/",
  "\\",
  "\t",
  "\n",
  "\r",
  ".",
  ".",
  "%",
  "2",
  "F",
  "e",
  "v",
  "i",
  "l",
  ":",
  "?",
  "#",
  "@",
  " ",
  "a",
  "\u0000",
  "　",
  "h",
  "t",
  "p",
  "s",
  "x",
  "=",
];
const APP_ORIGIN = "https://app.example";

describe("sanitizeReturnTo", () => {
  it("allows internal paths starting with /", () => {
    expect(sanitizeReturnTo("/cards/new")).toBe("/cards/new");
  });

  it("allows internal paths with query string", () => {
    expect(sanitizeReturnTo("/cards/new?foo=1")).toBe("/cards/new?foo=1");
  });

  it("rejects protocol-relative URLs starting with //", () => {
    expect(sanitizeReturnTo("//evil.com")).toBeNull();
  });

  it("rejects https:// URLs", () => {
    expect(sanitizeReturnTo("https://evil.com")).toBeNull();
  });

  it("rejects bare hostnames without leading slash", () => {
    expect(sanitizeReturnTo("evil.com")).toBeNull();
  });

  it("rejects undefined", () => {
    expect(sanitizeReturnTo(undefined)).toBeNull();
  });

  it("rejects empty string", () => {
    expect(sanitizeReturnTo("")).toBeNull();
  });

  it("rejects backslash-bypass /\\evil.com", () => {
    expect(sanitizeReturnTo("/\\evil.com")).toBeNull();
  });

  it("rejects backslash-bypass /\\\\evil.com", () => {
    // double-escaped to land "/\\evil.com" as the runtime string -- verify the helper
    // when called with the raw form a browser may emit
    expect(sanitizeReturnTo("/\\\\evil.com")).toBeNull();
  });

  it.each([
    ["/\t/evil.com", null],
    ["/\n/evil.com", null],
    ["/\r\\evil.com", null],
    ["//evil.com", null],
    ["/\\evil.com", null],
    ["https://x", null],
    ["/..//evil.com", null],
    ["/.%2e//evil.com", null],
    ["/ok?x=1#h", "/ok?x=1#h"],
    ["/", "/"],
    ["/cards new", "/cards%20new"],
  ])("sanitizeReturnTo(%j) returns %j", (input, expected) => {
    expect(sanitizeReturnTo(input)).toBe(expected);
  });

  it.each([[["/a", "//evil.com"]], [5], [{}]])("rejects non-string runtime value %j", (value) => {
    expect(sanitizeReturnTo(value as unknown as string)).toBeNull();
  });

  it("accepted values resolve to the app origin and are idempotent", () => {
    const rand = mulberry32(0x5eed);
    for (let i = 0; i < 5000; i++) {
      const length = Math.floor(rand() * 12);
      let v = "/";
      for (let j = 0; j < length; j++) {
        v += ALPHABET[Math.floor(rand() * ALPHABET.length)];
      }
      const r = sanitizeReturnTo(v);
      if (r === null) continue;
      const label = JSON.stringify(v);
      let origin: string;
      try {
        origin = new URL(r, APP_ORIGIN).origin;
      } catch (e) {
        throw new Error(`${label}: accepted value ${JSON.stringify(r)} failed to resolve: ${e}`);
      }
      expect(origin, label).toBe(APP_ORIGIN);
      expect(r.startsWith("/"), label).toBe(true);
      expect(sanitizeReturnTo(r), label).toBe(r);
    }
  });
});
