import type { NextConfig } from "next";

// Read BACKEND_URL directly from process.env here (not via src/env.ts) because Next
// loads this config file at build start, before the app bundle — and therefore before
// @t3-oss/env-nextjs's createEnv() — is ready. The Zod-validated facade in src/env.ts
// is for application code; production deploys must set BACKEND_URL explicitly so the
// localhost fallback below never reaches a real environment.
const backendUrl = process.env.BACKEND_URL ?? "http://localhost:1323";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: "/api/graphql",
        destination: `${backendUrl}/query`,
      },
    ];
  },
};

export default nextConfig;
