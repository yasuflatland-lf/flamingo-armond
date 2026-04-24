import type { NextConfig } from "next";

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
