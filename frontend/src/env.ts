// Validated server-side env facade. Importing this module triggers Zod validation at
// load time — any server component / route handler that reads BACKEND_URL through
// `env` will crash the build/process if the variable is missing or malformed.
// next.config.ts intentionally bypasses this (see that file for why).
import { createEnv } from "@t3-oss/env-nextjs";
import { z } from "zod";

export const env = createEnv({
  server: {
    BACKEND_URL: z.string().url(),
  },
  client: {},
  runtimeEnv: {
    BACKEND_URL: process.env.BACKEND_URL,
  },
  emptyStringAsUndefined: true,
});
