import { describe, expect, it } from "vitest";
import { assertBoundedTextLaw } from "@/test/text-arbitraries";
import { cardgroupSchema } from "./cardgroup";

describe("cardgroupSchema", () => {
  it("accepts 1..100 graphemes after Go TrimSpace and outputs the trimmed name (property)", () => {
    assertBoundedTextLaw(
      (name) => cardgroupSchema.safeParse({ name }),
      (d: { name: string }) => d.name,
      { max: 100, required: "name is required", tooLong: "name must be at most 100 characters" },
    );
  });

  it("rejects empty string", () => {
    const result = cardgroupSchema.safeParse({ name: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name is required");
    }
  });

  it("rejects a name of only U+0085", () => {
    const result = cardgroupSchema.safeParse({ name: "\u0085" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("name is required");
    }
  });

  it("accepts a name of only U+FEFF", () => {
    const result = cardgroupSchema.safeParse({ name: "\uFEFF" });
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.name).toBe("\uFEFF");
  });

  it("accepts exactly 100 graphemes", () => {
    const result = cardgroupSchema.safeParse({ name: "x".repeat(100) });
    expect(result.success).toBe(true);
  });

  it("rejects 101 graphemes", () => {
    const result = cardgroupSchema.safeParse({ name: "x".repeat(101) });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name must be at most 100 characters");
    }
  });

  it("counts ZWJ emoji as 1 grapheme (100 emoji = pass)", () => {
    const result = cardgroupSchema.safeParse({ name: "👨‍👩‍👧‍👦".repeat(100) });
    expect(result.success).toBe(true);
  });

  it("counts ZWJ emoji as 1 grapheme (101 emoji = fail)", () => {
    const result = cardgroupSchema.safeParse({ name: "👨‍👩‍👧‍👦".repeat(101) });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("name must be at most 100 characters");
    }
  });
});
