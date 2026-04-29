import { describe, expect, it } from "vitest";
import { newCardSchema, updateCardSchema } from "./card";

describe("newCardSchema", () => {
  it("accepts a valid card", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "group-1",
      front: "What is 2 + 2?",
      back: "4",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("What is 2 + 2?");
      expect(result.data.back).toBe("4");
      expect(result.data.cardgroupId).toBe("group-1");
    }
  });

  it("trims surrounding whitespace from front and back", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "group-1",
      front: "  question  ",
      back: "  answer  ",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("question");
      expect(result.data.back).toBe("answer");
    }
  });

  it("rejects empty cardgroupId", () => {
    const result = newCardSchema.safeParse({ cardgroupId: "", front: "q", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("cardgroupId is required");
    }
  });

  it("rejects empty front", () => {
    const result = newCardSchema.safeParse({ cardgroupId: "g1", front: "", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front is required");
    }
  });

  it("rejects all-whitespace front", () => {
    const result = newCardSchema.safeParse({ cardgroupId: "g1", front: "   ", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("front is required");
    }
  });

  it("rejects empty back", () => {
    const result = newCardSchema.safeParse({ cardgroupId: "g1", front: "q", back: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back is required");
    }
  });

  it("rejects all-whitespace back", () => {
    const result = newCardSchema.safeParse({ cardgroupId: "g1", front: "q", back: "   " });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("back is required");
    }
  });

  it("accepts exactly 500 graphemes for front", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "x".repeat(500),
      back: "a",
    });
    expect(result.success).toBe(true);
  });

  it("rejects 501 graphemes for front", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "x".repeat(501),
      back: "a",
    });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front must be at most 500 characters");
    }
  });

  it("accepts exactly 500 graphemes for back", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "q",
      back: "x".repeat(500),
    });
    expect(result.success).toBe(true);
  });

  it("rejects 501 graphemes for back", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "q",
      back: "x".repeat(501),
    });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back must be at most 500 characters");
    }
  });

  it("counts ZWJ emoji as 1 grapheme for front (500 emoji = pass)", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "👨‍👩‍👧‍👦".repeat(500),
      back: "a",
    });
    expect(result.success).toBe(true);
  });

  it("counts ZWJ emoji as 1 grapheme for front (501 emoji = fail)", () => {
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
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
    const result = newCardSchema.safeParse({
      cardgroupId: "g1",
      front: "q",
      back: "👨‍👩‍👧".repeat(500),
    });
    expect(result.success).toBe(true);
  });
});

describe("updateCardSchema", () => {
  it("accepts valid front and back", () => {
    const result = updateCardSchema.safeParse({ front: "updated q", back: "updated a" });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("updated q");
      expect(result.data.back).toBe("updated a");
    }
  });

  it("trims surrounding whitespace", () => {
    const result = updateCardSchema.safeParse({ front: "  q  ", back: "  a  " });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.front).toBe("q");
      expect(result.data.back).toBe("a");
    }
  });

  it("rejects empty front", () => {
    const result = updateCardSchema.safeParse({ front: "", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front is required");
    }
  });

  it("rejects all-whitespace front", () => {
    const result = updateCardSchema.safeParse({ front: "   ", back: "a" });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("front is required");
    }
  });

  it("rejects empty back", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back is required");
    }
  });

  it("rejects all-whitespace back", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "   " });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("back is required");
    }
  });

  it("accepts exactly 500 graphemes for front", () => {
    const result = updateCardSchema.safeParse({ front: "x".repeat(500), back: "a" });
    expect(result.success).toBe(true);
  });

  it("rejects 501 graphemes for front", () => {
    const result = updateCardSchema.safeParse({ front: "x".repeat(501), back: "a" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("front must be at most 500 characters");
    }
  });

  it("accepts exactly 500 graphemes for back", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "x".repeat(500) });
    expect(result.success).toBe(true);
  });

  it("rejects 501 graphemes for back", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "x".repeat(501) });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("back must be at most 500 characters");
    }
  });

  it("counts ZWJ emoji as 1 grapheme (500 emoji back = pass)", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "👨‍👩‍👧".repeat(500) });
    expect(result.success).toBe(true);
  });

  it("counts ZWJ emoji as 1 grapheme (501 emoji back = fail)", () => {
    const result = updateCardSchema.safeParse({ front: "q", back: "👨‍👩‍👧".repeat(501) });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("back must be at most 500 characters");
    }
  });
});
