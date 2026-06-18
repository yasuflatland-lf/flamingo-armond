import { describe, expect, it } from "vitest";
import { resolveHeaderSearchAction } from "./header-search-action";

describe("resolveHeaderSearchAction", () => {
  it("returns true on /cardgroups", () => {
    expect(resolveHeaderSearchAction("/cardgroups")).toBe(true);
  });

  it("returns false on non-filterable routes", () => {
    expect(resolveHeaderSearchAction("/")).toBe(false);
    expect(resolveHeaderSearchAction("/catalog")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/users")).toBe(false);
    expect(resolveHeaderSearchAction("/cardgroups/123/edit")).toBe(false);
    expect(resolveHeaderSearchAction("/learn/123")).toBe(false);
  });
});
