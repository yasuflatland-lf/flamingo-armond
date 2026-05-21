import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

describe("newRequestId — Web Crypto fallback path", () => {
  // Save a reference to the real crypto object so we can restore it after each test.
  const realCrypto = globalThis.crypto;

  beforeEach(() => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.resetModules();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    // Restore the real crypto global in case a test replaced it.
    vi.stubGlobal("crypto", realCrypto);
  });

  it("emits the module-load warn exactly once when crypto is undefined and still returns a UUID v7 string", async () => {
    // Remove the crypto global so the re-imported module sees it as unavailable.
    vi.stubGlobal("crypto", undefined);

    // Dynamic import after resetModules() triggers a fresh module evaluation.
    const { newRequestId: newRequestIdFresh } = await import("./request-id");

    // The module-load warn must have fired exactly once.
    expect(console.warn).toHaveBeenCalledTimes(1);
    expect(console.warn).toHaveBeenCalledWith(
      "[request-id] Web Crypto unavailable; falling back to Math.random-derived IDs (collision resistance reduced)",
    );

    // The function must still return a string matching the UUID v7 format.
    const id = newRequestIdFresh();
    expect(id).toMatch(UUID_V7_RE);
    expect(id[14]).toBe("7");
    expect(["8", "9", "a", "b"]).toContain(id[19]);
  });

  it("produces 20 unique values on the fallback path", async () => {
    vi.stubGlobal("crypto", undefined);
    const { newRequestId: newRequestIdFresh } = await import("./request-id");

    const ids = Array.from({ length: 20 }, newRequestIdFresh);
    const unique = new Set(ids);
    expect(unique.size).toBe(20);
  });

  it("timestamp prefix is non-decreasing across 100 calls on the fallback path", async () => {
    vi.stubGlobal("crypto", undefined);
    const { newRequestId: newRequestIdFresh } = await import("./request-id");

    const ids = Array.from({ length: 100 }, newRequestIdFresh);
    // The first 12 hex chars encode the 48-bit timestamp; they must be monotonically
    // non-decreasing. This guards against a future refactor that accidentally moves
    // the lastTimestampMs monotonicity guard inside the if (_cryptoAvailable) block.
    const prefixes = ids.map((id) => id.replace(/-/g, "").slice(0, 12));
    for (let i = 1; i < prefixes.length; i++) {
      const curr = prefixes[i] ?? "";
      const prev = prefixes[i - 1] ?? "";
      expect(curr >= prev).toBe(true);
    }
  });
});
