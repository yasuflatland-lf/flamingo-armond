# Frontend mutation testing (Stryker)

> Applies to: `frontend/`. Mutation testing measures *test effectiveness*, not
> code coverage. It is a diagnostic tool run on demand, not a per-PR gate.

## What it is

[Stryker](https://stryker-mutator.io/) mutates the source (flips `>` to `>=`,
drops a `!`, replaces a return value, …) and re-runs the test suite once per
mutant. The outcome per mutant:

- **Killed** — a test failed → the suite detects the change. Good.
- **Survived** — every test still passed → the suite cannot tell the mutated
  code from the original. This is a real coverage gap that line/branch coverage
  cannot surface: a line can be 100% executed yet have no assertion that pins
  its behaviour.
- **No coverage** — no test exercised the mutated line at all.

The headline number is the **mutation score** = killed / (killed + survived).

## Scope: `src/lib/**` only

`stryker.config.mjs` sets `mutate: ["src/lib/**/*.{ts,tsx}", "!**/*.test.*"]`.
This is the pure-logic layer (error parsers, sanitizers, formatters,
`auth-status`, CSP builders, …) with the highest unit-test density and the best
signal-to-noise ratio.

UI components and RSC pages are deliberately excluded. Their unit coverage is
thinner, so mutating them floods the report with low-value survived mutants and
multiplies the runtime (cost ≈ mutants × suite runtime). Widen `mutate` only
after the lib baseline is understood.

## Runner and config

- Test runner: `@stryker-mutator/vitest-runner`, driving the project's own
  `vitest.config.ts` (so path aliases, the `server-only` stub, and the per-test
  env all apply unchanged).
- `coverageAnalysis: "perTest"` — Stryker runs only the tests covering each
  mutant, the fastest mode the vitest runner supports.
- `incremental: true` — local reruns only re-test changed files; the cache lives
  in the gitignored `frontend/reports/mutation/`.

## Running it

```bash
# from the repo root
pnpm --filter frontend codegen        # src/lib/apollo/* needs src/generated/
pnpm --filter frontend test:mutation  # → frontend/reports/mutation/index.html
```

Open `frontend/reports/mutation/index.html` to browse survived mutants
file-by-file. `frontend/reports/` and `frontend/.stryker-tmp/` are gitignored.

## CI: report-only, off the PR path

`.github/workflows/frontend-mutation.yml` runs Stryker on `workflow_dispatch`
(manual) and a nightly `schedule`, never on PRs — mutation testing is too slow
for the fast-feedback loop. The HTML report is uploaded as the
`frontend-mutation-report` artifact.

The run is **report-only**: `thresholds.break` is `null` in `stryker.config.mjs`,
so a low score never fails the job. `high: 80` / `low: 60` only colour the report.

## Future enhancements

- **Adopt a `break` threshold.** Once a nightly baseline score is established,
  set `thresholds.break` to fail the job (and optionally a PR check) when the
  score regresses below it.
- **Add `@stryker-mutator/typescript-checker`.** It discards mutants that would
  not type-check, sharpening the score. Deferred initially because wiring `tsc`
  through the Next.js plugin / JSX / path aliases adds setup friction; revisit
  once the baseline run is green.
- **Widen `mutate`.** Extend beyond `src/lib/**` (e.g. `src/schemas/**`,
  `src/hooks/**`) as their unit coverage grows.
