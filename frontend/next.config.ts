import type { NextConfig } from "next";

// Read BACKEND_URL directly from process.env here (not via src/env.ts) because Next
// loads this config file at build start, before the app bundle — and therefore before
// @t3-oss/env-nextjs's createEnv() — is ready. The Zod-validated facade in src/env.ts
// is for application code; production deploys must set BACKEND_URL explicitly so the
// localhost fallback below never reaches a real environment.
const backendUrl = process.env.BACKEND_URL ?? "http://localhost:1323";

const htmlSecurityHeaders = [
  {
    key: "Strict-Transport-Security",
    // Conservative baseline: no preload and no subdomain coverage until deployment
    // ownership proves that scope is safe.
    value: "max-age=31536000",
  },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    value: "camera=(), geolocation=(), microphone=(), payment=(), usb=()",
  },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  {
    key: "Reporting-Endpoints",
    value: 'csp-endpoint="/api/csp-report"',
  },
];

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
  // Serve sw.js with no-cache so the browser revalidates it on every page load and
  // can byte-compare the response to detect Service Worker updates immediately.
  // Without no-cache the SW update algorithm throttles re-checks to at most once per
  // 24 hours (a Service Worker spec rule, independent of any HTTP cache TTL).
  async headers() {
    return [
      {
        source: "/((?!api/|_next/|sw\\.js$|offline\\.html$|.*\\.).*)",
        headers: htmlSecurityHeaders,
      },
      {
        source: "/offline.html",
        headers: [
          ...htmlSecurityHeaders,
          {
            key: "Content-Security-Policy",
            value:
              "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src 'self'; base-uri 'none'; form-action 'none'",
          },
        ],
      },
      {
        source: "/sw.js",
        headers: [
          { key: "Content-Type", value: "application/javascript; charset=utf-8" },
          { key: "Cache-Control", value: "no-cache, no-store, must-revalidate" },
          { key: "X-Content-Type-Options", value: "nosniff" },
        ],
      },
    ];
  },
};

export default nextConfig;
