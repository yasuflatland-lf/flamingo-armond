import { describe, expect, it } from "vitest";
import { assertBoundedTextLaw } from "@/test/text-arbitraries";
import { cardSchema } from "./card";

const SIDE = { max: 500 } as const;

describe("cardSchema", () => {
  it("front: accepts 1..500 graphemes after Go TrimSpace and outputs the trimmed text (property)", () => {
    assertBoundedTextLaw(
      (front) => cardSchema.safeParse({ front, back: "a" }),
      (d: { front: string }) => d.front,
      { ...SIDE, required: "front is required", tooLong: "front must be at most 500 characters" },
    );
  });

  it("back: accepts 1..500 graphemes after Go TrimSpace and outputs the trimmed text (property)", () => {
    assertBoundedTextLaw(
      (back) => cardSchema.safeParse({ front: "q", back }),
      (d: { back: string }) => d.back,
      { ...SIDE, required: "back is required", tooLong: "back must be at most 500 characters" },
    );
  });

  it("rejects empty front", () => {
    const result = cardSchema.safeParse({ front: "", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front is required");
    }
  });

  it("rejects a front of only U+0085", () => {
    const result = cardSchema.safeParse({ front: "\u0085", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("front is required");
    }
  });

  it("rejects empty back", () => {
    const result = cardSchema.safeParse({ front: "q", back: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back is required");
    }
  });

  it("accepts exactly 500 graphemes for front", () => {
    const result = cardSchema.safeParse({
      front: "x".repeat(500),
      back: "a",
    });
    expect(result.success).toBe(true);
  });

  it("rejects 501 graphemes for front", () => {
    const result = cardSchema.safeParse({
      front: "x".repeat(501),
      back: "a",
    });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front must be at most 500 characters");
    }
  });

  it("rejects 501 graphemes for back", () => {
    const result = cardSchema.safeParse({
      front: "q",
      back: "x".repeat(501),
    });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back must be at most 500 characters");
    }
  });

  it("counts ZWJ emoji as 1 grapheme for front (500 emoji = pass)", () => {
    const result = cardSchema.safeParse({
      front: "👨‍👩‍👧‍👦".repeat(500),
      back: "a",
    });
    expect(result.success).toBe(true);
  });

  it("counts ZWJ emoji as 1 grapheme for front (501 emoji = fail)", () => {
    const result = cardSchema.safeParse({
      front: "👨‍👩‍👧‍👦".repeat(501),
      back: "a",
    });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("front must be at most 500 characters");
    }
  });
});
