import { describe, expect, it } from "vitest";
import {
  ONBOARDING_COOKIE_TTL_SECONDS,
  signOnboardingCookie,
  verifyOnboardingCookie,
} from "./onboarding-cookie";

const SECRET = "test-secret-that-is-at-least-32-characters";
const OTHER_SECRET = "another-secret-that-is-also-32-chars-long!";
const SUB = "11111111-1111-4111-8111-111111111111";
const OTHER_SUB = "22222222-2222-4222-8222-222222222222";
const NOW = 1_700_000_000_000;

describe("onboarding cookie", () => {
  it("round-trips a value it minted for the same sub and secret", async () => {
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    await expect(verifyOnboardingCookie(value, SECRET, SUB, NOW)).resolves.toBe(true);
  });

  it("emits a versioned three-part value whose MAC is not the sub", async () => {
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    const parts = value.split(".");
    expect(parts).toHaveLength(3);
    expect(parts[0]).toBe("v1");
    expect(value).not.toContain(SUB);
  });

  it("rejects a value minted for a different sub (account switch)", async () => {
    const value = await signOnboardingCookie(SECRET, OTHER_SUB, NOW);
    await expect(verifyOnboardingCookie(value, SECRET, SUB, NOW)).resolves.toBe(false);
  });

  it("rejects a value minted under a different secret", async () => {
    const value = await signOnboardingCookie(OTHER_SECRET, SUB, NOW);
    await expect(verifyOnboardingCookie(value, SECRET, SUB, NOW)).resolves.toBe(false);
  });

  it("rejects a forged MAC — the fast path is not bypassable by hand-writing a cookie", async () => {
    const exp = Math.floor(NOW / 1000) + ONBOARDING_COOKIE_TTL_SECONDS;
    await expect(verifyOnboardingCookie(`v1.${exp}.forged`, SECRET, SUB, NOW)).resolves.toBe(false);
  });

  it("rejects a value once its embedded expiry has passed", async () => {
    const value = await signOnboardingCookie(SECRET, SUB, NOW);
    const afterTtl = NOW + (ONBOARDING_COOKIE_TTL_SECONDS + 1) * 1000;
    await expect(verifyOnboardingCookie(value, SECRET, SUB, afterTtl)).resolves.toBe(false);
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
