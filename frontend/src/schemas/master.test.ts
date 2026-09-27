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

  it("accepts a 500-grapheme description", () => {
    const r = masterSchema.safeParse({ name: "Deck", description: "a".repeat(500) });
    expect(r.success).toBe(true);
  });

  it("rejects a description longer than 500 grapheme clusters", () => {
    const r = masterSchema.safeParse({ name: "Deck", description: "a".repeat(501) });
    expect(r.success).toBe(false);
  });

  it("allows an empty description", () => {
    const r = masterSchema.safeParse({ name: "Deck", description: "" });
    expect(r.success).toBe(true);
  });

  it("accepts an empty sortOrder", () => {
    const r = masterSchema.safeParse({ name: "Deck", sortOrder: "" });
    expect(r.success).toBe(true);
  });

  it("accepts an integer sortOrder", () => {
    const r = masterSchema.safeParse({ name: "Deck", sortOrder: "5" });
    expect(r.success).toBe(true);
  });

  it("rejects exponent, hex and signed-plus notations", () => {
    for (const sortOrder of ["1e151", "1e0151", "0x10", "+5"]) {
      expect(masterSchema.safeParse({ name: "Deck", sortOrder }).success).toBe(false);
    }
  });

  it("rejects values outside int32", () => {
    for (const sortOrder of ["2147483648", "-2147483649", "99999999999"]) {
      expect(masterSchema.safeParse({ name: "Deck", sortOrder }).success).toBe(false);
    }
  });

  it("accepts the int32 bounds", () => {
    for (const sortOrder of ["2147483647", "-2147483648", " 7 "]) {
      expect(masterSchema.safeParse({ name: "Deck", sortOrder }).success).toBe(true);
    }
  });

  it("rejects a decimal sortOrder", () => {
    const r = masterSchema.safeParse({ name: "Deck", sortOrder: "1.5" });
    expect(r.success).toBe(false);
  });

  it("rejects a non-numeric sortOrder", () => {
    const r = masterSchema.safeParse({ name: "Deck", sortOrder: "abc" });
    expect(r.success).toBe(false);
  });
});
