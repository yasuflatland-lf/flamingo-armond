// Global fast-check parameters for every vitest run (plain `vitest` and Stryker).
// A fixed seed makes a red property reproducible and keeps Stryker's per-mutant
// killed/survived verdict stable across runs; set FC_SEED to an integer to explore
// other inputs locally. An empty FC_SEED keeps the default instead of silently
// becoming seed 0, and a non-integer fails loudly for the same reason.
// numRuns is fast-check's default, written out so it is not implicit.
import fc from "fast-check";

const raw = process.env.FC_SEED?.trim();
const seed = raw ? Number(raw) : 20260928;
if (!Number.isSafeInteger(seed)) {
  throw new Error(`FC_SEED must be an integer, got ${JSON.stringify(raw)}`);
}
fc.configureGlobal({ numRuns: 100, seed });
