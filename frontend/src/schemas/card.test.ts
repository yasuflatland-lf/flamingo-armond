import { describe, expect, it } from "vitest";
import { cardSchema } from "./card";

describe("cardSchema", () => {
  it("accepts a valid card", () => {
    const result = cardSchema.safeParse({
      front: "What is 2 + 2?",
      back: "4",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("What is 2 + 2?");
      expect(result.data.back).toBe("4");
    }
  });

  it("trims surrounding whitespace from front and back", () => {
    const result = cardSchema.safeParse({
      front: "  question  ",
      back: "  answer  ",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("question");
      expect(result.data.back).toBe("answer");
    }
  });

  it("rejects empty front", () => {
    const result = cardSchema.safeParse({ front: "", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front is required");
    }
  });

  it("rejects all-whitespace front", () => {
    const result = cardSchema.safeParse({ front: "   ", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("front is required");
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

  it("rejects all-whitespace back", () => {
    const result = cardSchema.safeParse({ front: "q", back: "   " });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("back is required");
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

  it("accepts exactly 500 graphemes for back", () => {
    const result = cardSchema.safeParse({
      front: "q",
      back: "x".repeat(500),
    });
    expect(result.success).toBe(true);
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

  it("counts ZWJ emoji as 1 grapheme for back (500 emoji = pass)", () => {
    const result = cardSchema.safeParse({
      front: "q",
      back: "👨‍👩‍👧".repeat(500),
    });
    expect(result.success).toBe(true);
  });
});
