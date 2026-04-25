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
});
