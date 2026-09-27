import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

async function loadEnvFresh(): Promise<typeof import("./env")> {
  return import("./env");
}

describe("env — NEXT_PUBLIC_SITE_URL", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("throws when NODE_ENV=production, VERCEL_ENV=production, and NEXT_PUBLIC_SITE_URL is unset", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");

    await expect(loadEnvFresh()).rejects.toThrow(/NEXT_PUBLIC_SITE_URL is required in production/);
  });

  it("does not throw when NODE_ENV=production, VERCEL_ENV=production, and NEXT_PUBLIC_SITE_URL is set", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://flamingo-armond-frontend.vercel.app");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("https://flamingo-armond-frontend.vercel.app");
  });

  it("falls back to https://<VERCEL_URL> when VERCEL_ENV=preview, NEXT_PUBLIC_SITE_URL is unset, and VERCEL_URL is set", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("VERCEL_ENV", "preview");
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    vi.stubEnv("VERCEL_URL", "flamingo-armond-git-feature-x.vercel.app");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("https://flamingo-armond-git-feature-x.vercel.app");
  });

  it("defaults to http://localhost:3000 outside production when NEXT_PUBLIC_SITE_URL is unset", async () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");

    const { env } = await loadEnvFresh();
    expect(env.NEXT_PUBLIC_SITE_URL).toBe("http://localhost:3000");
  });
});
