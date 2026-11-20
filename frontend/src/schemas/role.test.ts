import { describe, expect, it } from "vitest";
import { roleSchema } from "./role";

describe("roleSchema", () => {
  it("trims and lowercases like domain.ParseRoleName", () => {
    const result = roleSchema.safeParse({ name: "  Editor " });
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.name).toBe("editor");
  });

  it("maps U+0130 to i like Go strings.ToLower", () => {
    const result = roleSchema.safeParse({ name: "ADM\u0130N" });
    expect(result.success).toBe(true);
    if (result.success) expect(result.data.name).toBe("admin");
  });

  it("rejects an empty name after trim", () => {
    const result = roleSchema.safeParse({ name: " \u0085 " });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((i) => i.message)).toContain("name is required");
    }
  });

  it("rejects a name over 50 characters", () => {
    expect(roleSchema.safeParse({ name: "a".repeat(51) }).success).toBe(false);
    expect(roleSchema.safeParse({ name: "a".repeat(50) }).success).toBe(true);
  });

  it("rejects characters outside [a-z0-9_-]", () => {
    for (const name of ["a b", "\uFEFFa"]) {
      const result = roleSchema.safeParse({ name });
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues.map((i) => i.message)).toContain(
          "name must contain only lowercase letters, digits, '_' or '-'",
        );
      }
    }
  });
});
