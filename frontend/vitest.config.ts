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
    include: ["src/**/*.test.{ts,tsx}", "__tests__/**/*.test.{ts,tsx}"],
    // Expose vitest globals (describe, it, afterEach, etc.) so that
    // @testing-library/react can hook into afterEach for automatic DOM cleanup.
    globals: true,
    // Extend expect with jest-dom matchers for jsdom-based component tests
    setupFiles: ["src/__test-setup__/jest-dom.ts"],
    env: {
      BACKEND_URL: "http://localhost:1323",
      NEXT_PUBLIC_SUPABASE_URL: "http://localhost:54321",
      NEXT_PUBLIC_SUPABASE_ANON_KEY:
        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.vitest-dummy-key-1234567890",
    },
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/lib/**/*.ts"],
      exclude: ["src/generated/**", "src/**/*.test.{ts,tsx}"],
    },
  },
});
