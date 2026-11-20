import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { assertBoundedTextLaw, GO_SPACES } from "@/test/text-arbitraries";
import { masterDescriptionSchema, masterNameSchema, masterSortOrderSchema } from "./master";

const INT32_MIN = -(2n ** 31n);
const INT32_MAX = 2n ** 31n - 1n;

describe("masterNameSchema", () => {
  it("accepts 1..100 graphemes after Go TrimSpace and outputs the trimmed name (property)", () => {
    assertBoundedTextLaw(
      (name) => masterNameSchema.safeParse(name),
      (d: string) => d,
      { max: 100, required: "name is required", tooLong: "name must be at most 100 characters" },
    );
  });

  it("rejects a name longer than 100 grapheme clusters", () => {
    expect(masterNameSchema.safeParse("a".repeat(101)).success).toBe(false);
  });

  it("accepts a 100-grapheme name", () => {
    expect(masterNameSchema.safeParse("a".repeat(100)).success).toBe(true);
  });

  it("rejects a name of only U+0085", () => {
    expect(masterNameSchema.safeParse("\u0085").success).toBe(false);
  });

  it("accepts a name of only U+FEFF", () => {
    const r = masterNameSchema.safeParse("\uFEFF");
    expect(r.success).toBe(true);
    if (r.success) expect(r.data).toBe("\uFEFF");
  });
});

describe("masterDescriptionSchema", () => {
  it("accepts 0..500 graphemes after Go TrimSpace and outputs the trimmed text (property)", () => {
    assertBoundedTextLaw(
      (description) => masterDescriptionSchema.safeParse(description),
      (d: string) => d,
      { max: 500, tooLong: "description must be at most 500 characters" },
    );
  });

  it("accepts a 500-grapheme description", () => {
    expect(masterDescriptionSchema.safeParse("a".repeat(500)).success).toBe(true);
  });

  it("rejects a description longer than 500 grapheme clusters", () => {
    expect(masterDescriptionSchema.safeParse("a".repeat(501)).success).toBe(false);
  });

  it("counts description graphemes after Go trim", () => {
    const trailingNel = `${"a".repeat(500)}\u0085`;
    const trailingBom = `${"a".repeat(500)}\uFEFF`;
    expect(masterDescriptionSchema.safeParse(trailingNel).success).toBe(true);
    expect(masterDescriptionSchema.safeParse(trailingBom).success).toBe(false);
  });
});

describe("masterSortOrderSchema", () => {
  it("accepts an optionally padded base-10 integer iff it fits int32 (property)", () => {
    const pad = fc.string({ unit: fc.constantFrom(...GO_SPACES), maxLength: 2 });
    const n = fc.oneof(
      fc.bigInt({ min: INT32_MIN - 2n, max: INT32_MIN + 2n }),
      fc.bigInt({ min: INT32_MAX - 2n, max: INT32_MAX + 2n }),
      fc.bigInt({ min: -(2n ** 40n), max: 2n ** 40n }),
    );
    fc.assert(
      fc.property(pad, n, pad, (lead, value, trail) => {
        const ok = masterSortOrderSchema.safeParse(lead + value.toString() + trail).success;
        return ok === (value >= INT32_MIN && value <= INT32_MAX);
      }),
    );
  });

  it("accepts an empty or Go-whitespace-only value (property)", () => {
    fc.assert(
      fc.property(
        fc.string({ unit: fc.constantFrom(...GO_SPACES), maxLength: 3 }),
        (s) => masterSortOrderSchema.safeParse(s).success,
      ),
    );
  });

  it("rejects a decimal sortOrder", () => {
    expect(masterSortOrderSchema.safeParse("1.5").success).toBe(false);
  });

  it("rejects a non-numeric sortOrder", () => {
    expect(masterSortOrderSchema.safeParse("abc").success).toBe(false);
  });

  it("rejects exponent, hex and signed-plus notations", () => {
    for (const s of ["1e151", "1e0151", "0x10", "+5"]) {
      expect(masterSortOrderSchema.safeParse(s).success, s).toBe(false);
    }
  });

  it("rejects values outside int32", () => {
    for (const s of ["2147483648", "-2147483649", "99999999999"]) {
      expect(masterSortOrderSchema.safeParse(s).success, s).toBe(false);
    }
  });

  it("accepts the int32 bounds", () => {
    for (const s of ["2147483647", "-2147483648", " 7 "]) {
      expect(masterSortOrderSchema.safeParse(s).success, s).toBe(true);
    }
  });
});
