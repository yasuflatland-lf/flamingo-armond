// Validated env facade. Importing this module triggers Zod validation at load
// time. Server vars (BACKEND_URL) crash the build/process if missing or
// malformed; client vars (NEXT_PUBLIC_SUPABASE_*) fail the same way at the
// client bundle level. next.config.ts intentionally bypasses this (see that
// file for why).
import { createEnv } from "@t3-oss/env-nextjs";
import { z } from "zod";

export const env = createEnv({
  server: {
    BACKEND_URL: z.string().url(),
    // HMAC key for the middleware onboarding gate's fast-path cookie. Optional
    // so an unset deployment still behaves correctly — the gate then falls back
    // to a backend display-name lookup on every gated navigation instead of
    // trusting a cookie it cannot authenticate.
    ONBOARDING_GATE_SECRET: z.string().min(32).optional(),
  },
  client: {
    NEXT_PUBLIC_SUPABASE_URL: z.string().url(),
    NEXT_PUBLIC_SUPABASE_ANON_KEY: z.string().min(20),
    // Canonical site origin for SEO metadata (metadataBase, Open Graph URLs).
    // Falls back to localhost in development/test. On Vercel Preview, an unset
    // value is backfilled from the deployment's own VERCEL_URL (see runtimeEnv
    // below). Production (NODE_ENV=production && VERCEL_ENV=production) must
    // set it explicitly -- the module-level guard below throws otherwise.
    NEXT_PUBLIC_SITE_URL: z.string().url().default("http://localhost:3000"),
  },
  runtimeEnv: {
    BACKEND_URL: process.env.BACKEND_URL,
    ONBOARDING_GATE_SECRET: process.env.ONBOARDING_GATE_SECRET,
    NEXT_PUBLIC_SUPABASE_URL: process.env.NEXT_PUBLIC_SUPABASE_URL,
    NEXT_PUBLIC_SUPABASE_ANON_KEY: process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
    NEXT_PUBLIC_SITE_URL:
      // `||`, not `??`: emptyStringAsUndefined runs after this resolves, so an
      // empty-string value must also fall through to the Preview backfill.
      process.env.NEXT_PUBLIC_SITE_URL ||
      (process.env.VERCEL_ENV === "preview" && process.env.VERCEL_URL
        ? `https://${process.env.VERCEL_URL}`
        : undefined),
  },
  emptyStringAsUndefined: true,
});

// A production deploy that falls back to the localhost default silently ships
// a localhost robots.txt / sitemap.xml / Open Graph metadataBase. VERCEL_ENV is
// only "production" during an actual Vercel production build/runtime, so this
// never fires in local dev, CI unit tests, or Preview deploys.
if (
  process.env.NODE_ENV === "production" &&
  process.env.VERCEL_ENV === "production" &&
  !process.env.NEXT_PUBLIC_SITE_URL
) {
  throw new Error(
    "NEXT_PUBLIC_SITE_URL is required in production but is unset. Set it with " +
      "`vercel env add NEXT_PUBLIC_SITE_URL production` or in the Vercel " +
      "dashboard -- see docs/frontend/env-vars.md.",
  );
}
