// Global fast-check parameters for every vitest run (plain `vitest` and Stryker).
// A fixed seed makes a red property reproducible and keeps Stryker's per-mutant
// killed/survived verdict stable across runs; set FC_SEED to explore other inputs
// locally. numRuns is fast-check's default, written out so it is not implicit.
import fc from "fast-check";

fc.configureGlobal({ numRuns: 100, seed: Number(process.env.FC_SEED ?? "20260928") });
