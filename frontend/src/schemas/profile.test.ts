import { describe, expect, it } from "vitest";
import { updateProfileSchema } from "./profile";

describe("updateProfileSchema", () => {
  it("accepts a typical input", () => {
    const input = { displayName: "Alice", bio: "hello" };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.displayName).toBe("Alice");
      expect(result.data.bio).toBe("hello");
    }
  });

  it("trims displayName", () => {
    const input = { displayName: "  alice  ", bio: undefined };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.displayName).toBe("alice");
    }
  });

  it("rejects empty displayName", () => {
    const input = { displayName: "" };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("Display name is required");
    }
  });

  it("rejects displayName over 50 chars", () => {
    const input = { displayName: "x".repeat(51) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("Display name must be 50 characters or fewer");
    }
  });

  it("accepts undefined bio", () => {
    const input = { displayName: "Alice" };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.bio).toBeUndefined();
    }
  });

  it("accepts empty string bio (explicit clear)", () => {
    const input = { displayName: "Alice", bio: "" };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.bio).toBe("");
    }
  });

  it("rejects bio over 500 chars", () => {
    const input = { displayName: "Alice", bio: "x".repeat(501) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("Bio must be 500 characters or fewer");
    }
  });

  it("accepts emoji ZWJ family in displayName (50 graphemes)", () => {
    const input = { displayName: "👨‍👩‍👧‍👦".repeat(50) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
  });

  it("rejects displayName when 51 emoji ZWJ graphemes", () => {
    const input = { displayName: "👨‍👩‍👧‍👦".repeat(51) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(false);
    if (!result.success) {
      const messages = result.error.issues.map((i) => i.message);
      expect(messages).toContain("Display name must be 50 characters or fewer");
    }
  });

  it("accepts bio with 500 emoji ZWJ graphemes", () => {
    const input = { displayName: "Alice", bio: "👨‍👩‍👧".repeat(500) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(true);
  });

  it("rejects bio with 501 plain chars", () => {
    const input = { displayName: "Alice", bio: "a".repeat(501) };
    const result = updateProfileSchema.safeParse(input);
    expect(result.success).toBe(false);
    if (!result.success && result.error.issues[0]) {
      expect(result.error.issues[0].message).toBe("Bio must be 500 characters or fewer");
    }
  });

  it("rejects a reserved displayName (exact)", () => {
    const result = updateProfileSchema.safeParse({ displayName: "admin" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("Display name is reserved");
    }
  });

  it("rejects a reserved displayName case-insensitively", () => {
    const result = updateProfileSchema.safeParse({ displayName: "Admin" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("Display name is reserved");
    }
  });

  it("rejects a reserved displayName after trimming surrounding whitespace", () => {
    const result = updateProfileSchema.safeParse({ displayName: "  root  " });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("Display name is reserved");
    }
  });

  it("rejects the lowercased role-name 'general'", () => {
    const result = updateProfileSchema.safeParse({ displayName: "general" });
    expect(result.success).toBe(false);
  });

  it("accepts a non-reserved name that merely contains a reserved word (no substring match)", () => {
    const result = updateProfileSchema.safeParse({ displayName: "administrator2" });
    expect(result.success).toBe(true);
  });
});
