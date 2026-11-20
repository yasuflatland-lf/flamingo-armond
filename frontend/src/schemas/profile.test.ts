import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { assertBoundedTextLaw, paddedText } from "@/test/text-arbitraries";
import { updateProfileSchema } from "./profile";

describe("updateProfileSchema", () => {
  it("displayName: accepts 1..50 graphemes after Go TrimSpace and outputs the trimmed name (property)", () => {
    assertBoundedTextLaw(
      (displayName) => updateProfileSchema.safeParse({ displayName }),
      (d: { displayName: string }) => d.displayName,
      {
        max: 50,
        required: "Display name is required",
        tooLong: "Display name must be 50 characters or fewer",
      },
    );
  });

  it("bio: accepts iff at most 500 graphemes after Go TrimSpace and passes the raw value through (property)", () => {
    fc.assert(
      fc.property(paddedText(500), ({ raw, graphemes }) => {
        const r = updateProfileSchema.safeParse({ displayName: "Alice", bio: raw });
        return graphemes <= 500 ? r.success && r.data.bio === raw : !r.success;
      }),
    );
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

  it("accepts a 500-grapheme bio with trailing whitespace (backend trims first)", () => {
    for (const bio of [`${"x".repeat(500)} `, `${"x".repeat(500)}\n\n`]) {
      const result = updateProfileSchema.safeParse({ displayName: "Alice", bio });
      expect(result.success).toBe(true);
      if (result.success) expect(result.data.bio).toBe(bio);
    }
  });

  it("rejects a 500-grapheme bio followed by U+FEFF (not trimmed by Go)", () => {
    const result = updateProfileSchema.safeParse({
      displayName: "Alice",
      bio: `${"x".repeat(500)}\uFEFF`,
    });
    expect(result.success).toBe(false);
  });

  it("rejects a displayName of only U+0085 (Go TrimSpace strips it)", () => {
    const result = updateProfileSchema.safeParse({ displayName: "\u0085" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("Display name is required");
    }
  });

  it("accepts a displayName of only U+FEFF (Go TrimSpace keeps it)", () => {
    const result = updateProfileSchema.safeParse({ displayName: "\uFEFF" });
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.displayName).toBe("\uFEFF");
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

  it("rejects a reserved displayName written with U+0130", () => {
    for (const displayName of ["ADM\u0130N", "adm\u0130n"]) {
      const result = updateProfileSchema.safeParse({ displayName });
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.map((i) => i.message)).toContain("Display name is reserved");
      }
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
