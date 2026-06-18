import { describe, expect, it } from "vitest";
import { resolveHeaderSearchAction } from "./header-search-action";

describe("resolveHeaderSearchAction", () => {
  it("returns true on /cardgroups", () => {
    expect(resolveHeaderSearchAction("/cardgroups")).toBe(true);
  });

  it("returns true on the nested card-list edit routes", () => {
    expect(resolveHeaderSearchAction("/cardgroups/abc/edit")).toBe(true);
    expect(resolveHeaderSearchAction("/cardgroups/abc/edit/")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters/abc/edit")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters/abc/edit/")).toBe(true);
  });

  it("returns false on non-filterable routes", () => {
    expect(resolveHeaderSearchAction("/")).toBe(false);
    expect(resolveHeaderSearchAction("/catalog")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/users")).toBe(false);
    expect(resolveHeaderSearchAction("/learn/123")).toBe(false);
  });

  it("returns false on the list/detail siblings of the edit routes", () => {
    // Cardgroup detail (no /edit segment) and the masters list/detail are not
    // searchable card-list screens (the masters list filter is its own rollout).
    expect(resolveHeaderSearchAction("/cardgroups/abc")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/masters")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/masters/abc")).toBe(false);
  });
});
