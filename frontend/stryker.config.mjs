// Stryker mutation-testing configuration for the frontend.
//
// Mutation testing complements line/branch coverage: it mutates the source
// (e.g. flips a `>` to `>=`, drops a `!`) and re-runs the test suite for each
// mutant. A mutant the suite still passes ("survived") marks a gap the existing
// tests cannot detect. See `docs/frontend/mutation-testing.md`.
//
// Scope is deliberately limited to `src/lib/**` — the pure-logic layer with the
// highest unit-test coverage and the strongest signal-to-noise ratio. UI
// components and RSC pages are excluded because their unit coverage is thinner,
// which would flood the report with low-value survived mutants and inflate the
// runtime (mutants × suite runtime). Widen `mutate` once the lib baseline is
// understood.

/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
const config = {
  $schema: "./node_modules/@stryker-mutator/core/schema/stryker-schema.json",
  packageManager: "pnpm",
  testRunner: "vitest",
  // Declare the runner explicitly. The default plugin glob ("@stryker-mutator/*")
  // is resolved relative to stryker-core's own location, which under pnpm lives
  // in an isolated .pnpm store dir where the runner is not a sibling — so the
  // glob finds nothing and the run fails with "Cannot find TestRunner plugin".
  // Naming the package forces a direct import that pnpm resolves via the symlink.
  plugins: ["@stryker-mutator/vitest-runner"],
  // Reuse the project's vitest config so per-test env (BACKEND_URL, Supabase
  // dummies), path aliases, and the server-only stub all apply unchanged.
  vitest: {
    configFile: "vitest.config.ts",
  },
  // perTest coverage lets Stryker run only the tests that cover each mutant —
  // the fastest analysis mode the vitest runner supports.
  coverageAnalysis: "perTest",
  mutate: ["src/lib/**/*.{ts,tsx}", "!src/lib/**/*.test.{ts,tsx}"],
  reporters: ["html", "clear-text", "progress"],
  htmlReporter: {
    fileName: "reports/mutation/index.html",
  },
  // Report-only posture: `break: null` means a low mutation score never fails
  // the run. high/low only colour the report. Introduce a `break` threshold
  // once a baseline score is established (tracked in the topic doc).
  thresholds: {
    high: 80,
    low: 60,
    break: null,
  },
  // Cache mutant results so local reruns only re-test changed files. The
  // incremental file lives under the gitignored reports dir.
  incremental: true,
  incrementalFile: "reports/mutation/stryker-incremental.json",
};

export default config;
