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
});
