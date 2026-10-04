import fc from "fast-check";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { newRequestId } from "./request-id";

const UUID_V7_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

/**
 * For any clock readings (including a clock that steps backwards), a freshly
 * loaded module emits UUID v7 ids whose 48-bit timestamp prefix is the running
 * maximum of the readings so far.
 */
async function assertClockLaw(load: () => Promise<typeof newRequestId>): Promise<void> {
  const readings = fc.array(fc.integer({ min: 0, max: 2 ** 48 - 1 }), {
    minLength: 1,
    maxLength: 8,
  });
  await fc.assert(
    fc.asyncProperty(readings, async (clock) => {
      vi.resetModules();
      const fresh = await load();
      const now = vi.spyOn(Date, "now");
      let max = 0;
      try {
        for (const ms of clock) {
          now.mockReturnValueOnce(ms);
          max = Math.max(max, ms);
          const id = fresh();
          expect(id).toMatch(UUID_V7_RE);
          expect(id.replace(/-/g, "").slice(0, 12)).toBe(max.toString(16).padStart(12, "0"));
        }
      } finally {
        now.mockRestore();
      }
    }),
  );
}

describe("newRequestId", () => {
  it("produces a string matching the UUID v7 format regex", () => {
    expect(newRequestId()).toMatch(UUID_V7_RE);
  });

  it("produces 100 unique values", () => {
    const ids = Array.from({ length: 100 }, newRequestId);
    const unique = new Set(ids);
    expect(unique.size).toBe(100);
  });

  it("stamps the running maximum of Date.now() into the timestamp prefix (property)", async () => {
    await assertClockLaw(async () => (await import("./request-id")).newRequestId);
  });
});

describe("newRequestId — Web Crypto fallback path", () => {
  const realCrypto = globalThis.crypto;

  beforeEach(() => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.resetModules();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal("crypto", realCrypto);
  });

  async function loadFresh(): Promise<typeof import("./request-id")["newRequestId"]> {
    vi.stubGlobal("crypto", undefined);
    const { newRequestId: fn } = await import("./request-id");
    return fn;
  }

  it("emits the module-load warn exactly once when crypto is undefined and still returns a UUID v7 string", async () => {
    const newRequestIdFresh = await loadFresh();

    expect(console.warn).toHaveBeenCalledTimes(1);
    expect(console.warn).toHaveBeenCalledWith(
      "[request-id] Web Crypto unavailable; falling back to Math.random-derived IDs (collision resistance reduced)",
    );

    const id = newRequestIdFresh();
    expect(id).toMatch(UUID_V7_RE);
    expect(id[14]).toBe("7");
    expect(["8", "9", "a", "b"]).toContain(id[19]);
  });

  it("produces 20 unique values on the fallback path", async () => {
    const newRequestIdFresh = await loadFresh();

    const ids = Array.from({ length: 20 }, newRequestIdFresh);
    expect(new Set(ids).size).toBe(20);
  });

  it("stamps the running maximum of Date.now() into the timestamp prefix on the fallback path (property)", async () => {
    // Guards against accidentally moving the lastTimestampMs monotonicity guard
    // inside the if (_cryptoAvailable) block.
    await assertClockLaw(loadFresh);
  });
});
