import { describe, expect, it } from "vitest";
import { masterSchema } from "./master";

describe("masterSchema", () => {
  it("accepts a valid name and trims it", () => {
    const r = masterSchema.safeParse({ name: "  Spanish A1  " });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.name).toBe("Spanish A1");
  });

  it("rejects an empty name after trim", () => {
    const r = masterSchema.safeParse({ name: "   " });
    expect(r.success).toBe(false);
  });

  it("rejects a name longer than 100 grapheme clusters", () => {
    const r = masterSchema.safeParse({ name: "a".repeat(101) });
    expect(r.success).toBe(false);
  });

  it("accepts a 100-grapheme name", () => {
    const r = masterSchema.safeParse({ name: "a".repeat(100) });
    expect(r.success).toBe(true);
  });

  it("collapses an empty optional text field to null", () => {
    const r = masterSchema.safeParse({ name: "Deck", description: "   " });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.description).toBeNull();
  });

  it("accepts a 1000-grapheme description and rejects 1001", () => {
    expect(masterSchema.safeParse({ name: "Deck", description: "a".repeat(1000) }).success).toBe(
      true,
    );
    expect(masterSchema.safeParse({ name: "Deck", description: "a".repeat(1001) }).success).toBe(
      false,
    );
  });

  it("rejects a language longer than 50", () => {
    expect(masterSchema.safeParse({ name: "Deck", language: "a".repeat(51) }).success).toBe(false);
  });

  it("accepts a valid https coverImageUrl", () => {
    const r = masterSchema.safeParse({ name: "Deck", coverImageUrl: "https://example.com/a.png" });
    expect(r.success).toBe(true);
  });

  it("rejects a javascript: coverImageUrl", () => {
    expect(
      masterSchema.safeParse({ name: "Deck", coverImageUrl: "javascript:alert(1)" }).success,
    ).toBe(false);
  });

  it("rejects a non-url coverImageUrl", () => {
    expect(masterSchema.safeParse({ name: "Deck", coverImageUrl: "not a url" }).success).toBe(
      false,
    );
  });

  it("accepts an integer sortOrder string and a negative one", () => {
    expect(masterSchema.safeParse({ name: "Deck", sortOrder: "5" }).success).toBe(true);
    expect(masterSchema.safeParse({ name: "Deck", sortOrder: "-3" }).success).toBe(true);
  });

  it("rejects a non-integer sortOrder string", () => {
    expect(masterSchema.safeParse({ name: "Deck", sortOrder: "1.5" }).success).toBe(false);
    expect(masterSchema.safeParse({ name: "Deck", sortOrder: "abc" }).success).toBe(false);
  });

  it("treats an empty sortOrder string as null", () => {
    const r = masterSchema.safeParse({ name: "Deck", sortOrder: "" });
    expect(r.success).toBe(true);
    if (r.success) expect(r.data.sortOrder).toBeNull();
  });
});
