import fc from "fast-check";
import { describe, expect, it } from "vitest";
import {
  ONBOARDING_COOKIE_TTL_SECONDS,
  signOnboardingCookie,
  verifyOnboardingCookie,
} from "./onboarding-cookie";

const SECRET = "test-secret-that-is-at-least-32-characters";
const SUB = "11111111-1111-4111-8111-111111111111";
const NOW = 1_700_000_000_000;
const TTL = ONBOARDING_COOKIE_TTL_SECONDS;

// Not minLength 0: Web Crypto rejects a zero-length HMAC key.
const nonEmpty = fc.string({ minLength: 1 });
// Not sub + suffix: a strict superstring always changes the length, so a MAC
// that binds only the length would pass.
const distinctPair = fc.tuple(nonEmpty, nonEmpty).filter(([a, b]) => a !== b);

describe("onboarding cookie", () => {
  it("verifies its own value exactly while now sits in [exp - TTL, exp) (property)", async () => {
    await fc.assert(
      fc.asyncProperty(
        nonEmpty,
        nonEmpty,
        fc.integer({ min: 1e12, max: 2e12 }),
        // Not one uniform ±TTL arm alone: it lands on the exact expiry second
        // ~1/7200 of the time, so 100 runs would miss the exp <= now boundary.
        fc.oneof(
          fc.integer({ min: -(TTL + 10) * 1000, max: (TTL + 10) * 1000 }),
          fc.integer({ min: -1500, max: 1500 }),
          fc.integer({ min: TTL * 1000 - 1500, max: TTL * 1000 + 1500 }),
        ),
        async (secret, sub, t, dt) => {
          const value = await signOnboardingCookie(secret, sub, t);
          const exp = Math.floor(t / 1000) + TTL;
          const now = Math.floor((t + dt) / 1000);
          const fresh = exp > now && exp <= now + TTL;
          expect(await verifyOnboardingCookie(value, secret, sub, t + dt)).toBe(fresh);
        },
      ),
    );
  });

  it("rejects its value for any other sub, any other secret, or any one-character edit (property)", async () => {
    await fc.assert(
      fc.asyncProperty(
        distinctPair,
        distinctPair,
        fc.nat(),
        async ([secret, otherSecret], [sub, otherSub], at) => {
          const value = await signOnboardingCookie(secret, sub, NOW);
          const i = at % value.length;
          const edited = value.slice(0, i) + (value[i] === "A" ? "B" : "A") + value.slice(i + 1);
          expect(await verifyOnboardingCookie(value, secret, otherSub, NOW)).toBe(false);
          expect(await verifyOnboardingCookie(value, otherSecret, sub, NOW)).toBe(false);
          expect(await verifyOnboardingCookie(edited, secret, sub, NOW)).toBe(false);
        },
      ),
    );
  });

  it("emits a versioned three-part value whose MAC is not the sub", async () => {
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    const parts = value.split(".");
    expect(parts).toHaveLength(3);
    expect(parts[0]).toBe("v1");
    expect(value).not.toContain(SUB);
  });

  it("rejects a forged MAC — the fast path is not bypassable by hand-writing a cookie", async () => {
    const exp = Math.floor(NOW / 1000) + ONBOARDING_COOKIE_TTL_SECONDS;
    await expect(verifyOnboardingCookie(`v1.${exp}.forged`, SECRET, SUB, NOW)).resolves.toBe(false);
  });

  it("rejects a value whose expiry sits further out than the current TTL allows", async () => {
    const beforeTtl = NOW - (ONBOARDING_COOKIE_TTL_SECONDS + 60) * 1000;
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    // Verifying "in the past" models a shortened TTL: exp is still authentic but
    // now describes a longer lifetime than policy permits.
    await expect(verifyOnboardingCookie(value, SECRET, SUB, beforeTtl)).resolves.toBe(false);
  });

  it.each([
    ["undefined", undefined],
    ["empty", ""],
    ["one part", "v1"],
    ["two parts", "v1.1700003600"],
    ["four parts", "v1.1700003600.mac.extra"],
    ["wrong version", "v2.1700003600.mac"],
    ["non-numeric expiry", "v1.not-a-number.mac"],
  ])("rejects a malformed value (%s)", async (_label, value) => {
    await expect(verifyOnboardingCookie(value, SECRET, SUB, NOW)).resolves.toBe(false);
  });

  it("rejects verification with an empty sub", async () => {
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    await expect(verifyOnboardingCookie(value, SECRET, "", NOW)).resolves.toBe(false);
  });
});
