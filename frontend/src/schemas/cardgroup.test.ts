import { describe, expect, it } from "vitest";
import { newCardgroupSchema, updateCardgroupSchema } from "./cardgroup";

describe("newCardgroupSchema", () => {
  it("accepts a valid name", () => {
    const result = newCardgroupSchema.safeParse({ name: "My Group" });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.name).toBe("My Group");
    }
  });

  it("trims surrounding whitespace from name", () => {
    const result = newCardgroupSchema.safeParse({ name: "  trimmed  " });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.name).toBe("trimmed");
    }
  });

  it("rejects empty string", () => {
    const result = newCardgroupSchema.safeParse({ name: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name is required");
    }
  });

  it("rejects all-whitespace string", () => {
    const result = newCardgroupSchema.safeParse({ name: "   " });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("name is required");
    }
  });

  it("accepts exactly 100 graphemes", () => {
    const result = newCardgroupSchema.safeParse({ name: "x".repeat(100) });
    expect(result.success).toBe(true);
  });

  it("rejects 101 graphemes", () => {
    const result = newCardgroupSchema.safeParse({ name: "x".repeat(101) });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name must be at most 100 characters");
    }
  });

  it("counts ZWJ emoji as 1 grapheme (100 emoji = pass)", () => {
    const result = newCardgroupSchema.safeParse({ name: "👨‍👩‍👧‍👦".repeat(100) });
    expect(result.success).toBe(true);
  });

  it("counts ZWJ emoji as 1 grapheme (101 emoji = fail)", () => {
    const result = newCardgroupSchema.safeParse({ name: "👨‍👩‍👧‍👦".repeat(101) });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("name must be at most 100 characters");
    }
  });
});

describe("updateCardgroupSchema", () => {
  it("accepts a valid name", () => {
    const result = updateCardgroupSchema.safeParse({ name: "Updated Group" });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.name).toBe("Updated Group");
    }
  });

  it("rejects empty string", () => {
    const result = updateCardgroupSchema.safeParse({ name: "" });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name is required");
    }
  });

  it("rejects all-whitespace string", () => {
    const result = updateCardgroupSchema.safeParse({ name: "   " });
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("name is required");
    }
  });

  it("accepts exactly 100 graphemes", () => {
    const result = updateCardgroupSchema.safeParse({ name: "x".repeat(100) });
    expect(result.success).toBe(true);
  });

  it("rejects 101 graphemes", () => {
    const result = updateCardgroupSchema.safeParse({ name: "x".repeat(101) });
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("name must be at most 100 characters");
    }
  });
});
