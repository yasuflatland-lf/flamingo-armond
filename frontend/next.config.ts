import type { NextConfig } from "next";

// Read BACKEND_URL directly from process.env here (not via src/env.ts) because Next
// loads this config file at build start, before the app bundle — and therefore before
// @t3-oss/env-nextjs's createEnv() — is ready. The Zod-validated facade in src/env.ts
// is for application code; production deploys must set BACKEND_URL explicitly so the
// localhost fallback below never reaches a real environment.
const backendUrl = process.env.BACKEND_URL ?? "http://localhost:1323";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Next 16's blockCrossSiteDEV allowlists `localhost` but not `127.0.0.1`,
  // and Google OAuth requires us to access dev via 127.0.0.1 (see docs/dev-setup.md).
  // Without this, /_next/webpack-hmr WebSocket upgrades return 403 "Unauthorized".
  allowedDevOrigins: ["127.0.0.1"],
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
