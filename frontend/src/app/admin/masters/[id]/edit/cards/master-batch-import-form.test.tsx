// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { resolveStep1Button } from "./master-batch-import-form";

describe("MasterBatchImportForm / resolveStep1Button", () => {
  it("disables the button with no text", () => {
    expect(
      resolveStep1Button({ hasText: false, validating: false, result: null, isStale: false }),
    ).toEqual({ labelKey: "validate", action: null, disabled: true });
  });

  it("offers continue when a valid non-stale result exists", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [], errors: [] },
        isStale: false,
      }),
    ).toEqual({ labelKey: "import", action: "continue", disabled: false });
  });

  it("forces re-validate when the validated payload is stale", () => {
    expect(
      resolveStep1Button({
        hasText: true,
        validating: false,
        result: { valid: true, parsedCards: [], errors: [] },
        isStale: true,
      }),
    ).toEqual({ labelKey: "validate", action: "validate", disabled: false });
  });

  it("does not carry optimisticResponse (static-source guard)", () => {
    const src = readFileSync(
      join(process.cwd(), "src/app/admin/masters/[id]/edit/cards/master-batch-import-form.tsx"),
      "utf8",
    );
    expect(src).toContain("adminImportMasterCards");
    expect(src).not.toContain("importCards(");
  });
});
