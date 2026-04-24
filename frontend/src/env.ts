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
  },
  client: {
    NEXT_PUBLIC_SUPABASE_URL: z.string().url(),
    NEXT_PUBLIC_SUPABASE_ANON_KEY: z.string().min(20),
  },
  runtimeEnv: {
    BACKEND_URL: process.env.BACKEND_URL,
    NEXT_PUBLIC_SUPABASE_URL: process.env.NEXT_PUBLIC_SUPABASE_URL,
    NEXT_PUBLIC_SUPABASE_ANON_KEY: process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
  },
  emptyStringAsUndefined: true,
});
