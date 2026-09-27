// @vitest-environment node
import { describe, expect, it } from "vitest";
import { encodeUtf8Base64 } from "./encode-utf8-base64";

function legacyEncode(text: string): string {
  return btoa(unescape(encodeURIComponent(text)));
}

function decodeBase64Utf8(encoded: string): string {
  return new TextDecoder().decode(Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0)));
}

describe("encodeUtf8Base64", () => {
  it("matches the legacy btoa(unescape(encodeURIComponent)) output for well-formed text", () => {
    const inputs = [
      "",
      "abc",
      "hola\tadios\n",
      "\u65e5\u672c\u8a9e\t\u306b\u307b\u3093\u3054",
      "\u{1f600}\tx",
      "a".repeat(200_000),
    ];
    for (const s of inputs) {
      expect(encodeUtf8Base64(s), JSON.stringify(s.slice(0, 40))).toBe(legacyEncode(s));
    }
  });

  it("replaces a lone surrogate with U+FFFD instead of throwing", () => {
    expect(encodeUtf8Base64("\ude00")).toBe("77+9");
    expect(encodeUtf8Base64("a\ud83d")).toBe("Ye+/vQ==");
    expect(encodeUtf8Base64("a\tb\ude00")).toBe("YQli77+9");
  });

  it("round-trips to the TextEncoder/TextDecoder normalisation for arbitrary DOM strings", () => {
    const units = ["a", "\t", "\n", "\u3042", "\u{1f600}", "\ud83d", "\ude00", "\u00e9"];
    let seed = 0x5eed;
    function next(): number {
      // Math.imul keeps the multiply exact; a plain `*` loses low bits past 2^53.
      seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
      return seed >>> 16;
    }
    for (let n = 0; n < 500; n++) {
      const length = next() % 13;
      let s = "";
      for (let i = 0; i < length; i++) {
        s += units[next() % units.length];
      }
      const expected = new TextDecoder().decode(new TextEncoder().encode(s));
      expect(decodeBase64Utf8(encodeUtf8Base64(s)), JSON.stringify(s)).toBe(expected);
    }
  });
});
