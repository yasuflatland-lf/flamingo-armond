import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

async function loadEnvFresh(): Promise<typeof import("./env")> {
  return import("./env");
}

// The guard and the Preview backfill read raw process.env before
// emptyStringAsUndefined runs, so an empty string and an absent key
// (stubEnv with undefined deletes it) are distinct inputs to pin.
const UNSET_FORMS = [
  ["an empty string", ""],
  ["absent", undefined],
] as const;

describe("env — NEXT_PUBLIC_SITE_URL", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it.each(UNSET_FORMS)(
    "throws when NODE_ENV=production, VERCEL_ENV=production, and NEXT_PUBLIC_SITE_URL is %s",
    async (_label, value) => {
      vi.stubEnv("NODE_ENV", "production");
      vi.stubEnv("VERCEL_ENV", "production");
      vi.stubEnv("NEXT_PUBLIC_SITE_URL", value);

      await expect(loadEnvFresh()).rejects.toThrow(
        /NEXT_PUBLIC_SITE_URL is required in production/,
      );
    },
  );

  it("does not throw when NODE_ENV=production, VERCEL_ENV=production, and NEXT_PUBLIC_SITE_URL is set", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://flamingo-armond-frontend.vercel.app");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("https://flamingo-armond-frontend.vercel.app");
  });

  it.each(UNSET_FORMS)(
    "falls back to https://<VERCEL_URL> when VERCEL_ENV=preview, NEXT_PUBLIC_SITE_URL is %s, and VERCEL_URL is set",
    async (_label, value) => {
      vi.stubEnv("NODE_ENV", "production");
      vi.stubEnv("VERCEL_ENV", "preview");
      vi.stubEnv("NEXT_PUBLIC_SITE_URL", value);
      vi.stubEnv("VERCEL_URL", "flamingo-armond-git-feature-x.vercel.app");

      const { env } = await loadEnvFresh();
      expect(env.NEXT_PUBLIC_SITE_URL).toBe("https://flamingo-armond-git-feature-x.vercel.app");
    },
  );

  it.each(UNSET_FORMS)(
    "defaults to http://localhost:3000 outside production when NEXT_PUBLIC_SITE_URL is %s",
    async (_label, value) => {
      vi.stubEnv("NEXT_PUBLIC_SITE_URL", value);

      const { env } = await loadEnvFresh();
      expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
    },
  );
});
