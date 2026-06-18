import { describe, expect, it } from "vitest";
import { resolveHeaderSearchAction } from "./header-search-action";

describe("resolveHeaderSearchAction", () => {
  it("returns true on filterable list routes", () => {
    expect(resolveHeaderSearchAction("/cardgroups")).toBe(true);
    expect(resolveHeaderSearchAction("/catalog")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/users")).toBe(true);
  });

  it("returns false on non-filterable routes", () => {
    expect(resolveHeaderSearchAction("/")).toBe(false);
    expect(resolveHeaderSearchAction("/cardgroups/123/edit")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/masters/123/edit")).toBe(false);
    expect(resolveHeaderSearchAction("/learn/123")).toBe(false);
  });
});
