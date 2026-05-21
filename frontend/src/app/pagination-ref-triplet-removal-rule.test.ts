import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const productionSites = [
  {
    name: "cardgroups listing",
    sourcePath: "src/app/cardgroups/cardgroups-client.tsx",
  },
  {
    name: "cardgroup cards connection hook",
    sourcePath: "src/app/cardgroups/[id]/cards/use-cards-connection.ts",
  },
  {
    name: "admin users listing",
    sourcePath: "src/app/admin/users/AdminUsersClient.tsx",
  },
];

describe("pagination useEffectEvent migration contract", () => {
  it.each(
    productionSites,
  )("$name uses useEffectEvent instead of the cursor/search/hasNextPage ref triplet", ({
    sourcePath,
  }) => {
    const source = readFileSync(join(process.cwd(), sourcePath), "utf8");

    expect(source).toContain("useEffectEvent");
    expect(source).not.toMatch(/\b(endCursorRef|hasNextPageRef|searchQueryRef)\b/);
  });
});
