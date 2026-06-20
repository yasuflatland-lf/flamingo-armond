import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

describe("useEntityCardMutations", () => {
  // The generic hook is the single owner of the four card mutations' calls, so
  // it is the canonical location of the optimisticResponse-absence guard (the
  // wrapper guards now sit over thin config files with no mutation calls).
  // Typed GraphQL errors (…DuplicateFrontError, InputValidationError, FORBIDDEN)
  // can fail these mutations and Apollo does not reliably roll back optimistic
  // writes for typed errors. See .claude/rules/pagination.md
  // § "Drop `optimisticResponse` for mutations that can fail with typed GraphQL errors".
  it("does not pass optimisticResponse as a mutation option (static-source guard)", () => {
    const src = readFileSync(
      join(process.cwd(), "src/lib/cards/use-entity-card-mutations.ts"),
      "utf8",
    );
    // Matches the option key `optimisticResponse:`; tolerates the documenting
    // comment word `optimisticResponse` (no colon).
    expect(src).not.toMatch(/optimisticResponse\s*:/);
  });
});
