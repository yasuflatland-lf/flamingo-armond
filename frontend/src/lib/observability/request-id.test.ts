import { describe, expect, it } from "vitest";
import { newRequestId } from "./request-id";

const UUID_V7_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("newRequestId", () => {
  it("produces a string matching the UUID v7 format regex", () => {
    expect(newRequestId()).toMatch(UUID_V7_RE);
  });

  it("has version nibble '7' at position 14", () => {
    const id = newRequestId();
    expect(id[14]).toBe("7");
  });

  it("has variant nibble in {8,9,a,b} at position 19", () => {
    const id = newRequestId();
    expect(["8", "9", "a", "b"]).toContain(id[19]);
  });

  it("produces 100 unique values", () => {
    const ids = Array.from({ length: 100 }, newRequestId);
    const unique = new Set(ids);
    expect(unique.size).toBe(100);
  });

  it("timestamp prefix is non-decreasing across 100 calls", () => {
    const ids = Array.from({ length: 100 }, newRequestId);
    // The first 12 hex chars encode the 48-bit timestamp; they must be monotonically non-decreasing.
    const prefixes = ids.map((id) => id.replace(/-/g, "").slice(0, 12));
    for (let i = 1; i < prefixes.length; i++) {
      const curr = prefixes[i] ?? "";
      const prev = prefixes[i - 1] ?? "";
      expect(curr >= prev).toBe(true);
    }
  });
});
