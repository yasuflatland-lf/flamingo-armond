import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? [["html"], ["github"], ["list"]] : [["html"], ["list"]],
  use: {
    baseURL,
    locale: "ja-JP",
    timezoneId: "Asia/Tokyo",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  // webServer runs `next build` then `next start` so tests exercise the same code
  // path as production. `reuseExistingServer: !process.env.CI` lets a developer
  // running `next dev` on :3000 skip both build and start. The `env` block falls
  // back from NEXT_PUBLIC_SUPABASE_* to E2E_SUPABASE_* so a CI runner exporting
  // only the E2E_* form still satisfies @t3-oss/env-nextjs build-time validation.
  webServer: {
    command: "pnpm --filter frontend build && pnpm --filter frontend start",
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: {
      BACKEND_URL: process.env.BACKEND_URL ?? "http://localhost:1323",
      NEXT_PUBLIC_SUPABASE_URL:
        process.env.NEXT_PUBLIC_SUPABASE_URL ?? process.env.E2E_SUPABASE_URL ?? "",
      NEXT_PUBLIC_SUPABASE_ANON_KEY:
        process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? process.env.E2E_SUPABASE_ANON_KEY ?? "",
    },
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        launchOptions: {
          args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
        },
      },
    },
  ],
});
