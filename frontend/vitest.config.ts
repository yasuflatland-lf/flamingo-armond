import { resolve } from "node:path";
import tsconfigPaths from "vite-tsconfig-paths";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [tsconfigPaths()],
  resolve: {
    alias: {
      // stub Next.js server-only guard so vitest can import server modules
      "server-only": resolve(__dirname, "src/__mocks__/server-only.ts"),
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    env: {
      BACKEND_URL: "http://localhost:1323",
    },
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/lib/**/*.ts"],
      exclude: ["src/generated/**", "src/**/*.test.ts"],
    },
  },
});
