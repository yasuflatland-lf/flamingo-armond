import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

async function loadEnvFresh(): Promise<typeof import("./env")> {
  return import("./env");
}

// The guard and the Preview backfill read raw process.env, which
// emptyStringAsUndefined never normalises, so an empty string and an absent key
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

  it("prefers an explicit NEXT_PUBLIC_SITE_URL over the VERCEL_URL backfill when VERCEL_ENV=preview", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "preview");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://flamingo-armond-frontend.vercel.app");
    vi.stubEnv("VERCEL_URL", "flamingo-armond-git-feature-x.vercel.app");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("https://flamingo-armond-frontend.vercel.app");
  });

  it("falls back to http://localhost:3000 when VERCEL_ENV=preview, NEXT_PUBLIC_SITE_URL is unset, and VERCEL_URL is absent", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "preview");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    vi.stubEnv("VERCEL_URL", undefined);

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
  });

  it("ignores VERCEL_URL outside Preview when NEXT_PUBLIC_SITE_URL is unset", async () => {
    vi.stubEnv("VERCEL_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    vi.stubEnv("VERCEL_URL", "flamingo-armond-git-feature-x.vercel.app");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
  });

  it("does not throw when VERCEL_ENV=production but NODE_ENV is not production", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("VERCEL_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
  });

  it.each(UNSET_FORMS)(
    "defaults to http://localhost:3000 outside production when NEXT_PUBLIC_SITE_URL is %s",
    async (_label, value) => {
      vi.stubEnv("NEXT_PUBLIC_SITE_URL", value);

      const { env } = await loadEnvFresh();
      expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
    },
  );
});
