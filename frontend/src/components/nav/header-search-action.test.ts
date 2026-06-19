import { describe, expect, it } from "vitest";
import { resolveHeaderSearchAction } from "./header-search-action";

describe("resolveHeaderSearchAction", () => {
  it("returns true on filterable list routes", () => {
    expect(resolveHeaderSearchAction("/cardgroups")).toBe(true);
    expect(resolveHeaderSearchAction("/catalog")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/users")).toBe(true);
  });

  it("returns true on the nested card-list edit routes", () => {
    expect(resolveHeaderSearchAction("/cardgroups/abc/edit")).toBe(true);
    expect(resolveHeaderSearchAction("/cardgroups/abc/edit/")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters/abc/edit")).toBe(true);
    expect(resolveHeaderSearchAction("/admin/masters/abc/edit/")).toBe(true);
  });

  it("returns false on non-filterable routes", () => {
    expect(resolveHeaderSearchAction("/")).toBe(false);
    expect(resolveHeaderSearchAction("/learn/123")).toBe(false);
  });

  it("returns false on the detail (non-edit) routes", () => {
    // The detail routes carry no /edit segment, so they are not searchable —
    // only each list route and its [id]/edit card-list route are. (The masters
    // list route /admin/masters is itself searchable, so it is not listed here.)
    expect(resolveHeaderSearchAction("/cardgroups/abc")).toBe(false);
    expect(resolveHeaderSearchAction("/admin/masters/abc")).toBe(false);
  });
});
