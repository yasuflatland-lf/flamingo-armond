import { describe, expect, it } from "vitest";
import { shouldShowHeaderSearch } from "./header-search-action";

describe("shouldShowHeaderSearch", () => {
  it("returns true on filterable list routes", () => {
    expect(shouldShowHeaderSearch("/cardgroups")).toBe(true);
    expect(shouldShowHeaderSearch("/catalog")).toBe(true);
    expect(shouldShowHeaderSearch("/admin/masters")).toBe(true);
    expect(shouldShowHeaderSearch("/admin/users")).toBe(true);
  });

  it("returns true on the nested card-list edit routes", () => {
    expect(shouldShowHeaderSearch("/cardgroups/abc/edit")).toBe(true);
    expect(shouldShowHeaderSearch("/cardgroups/abc/edit/")).toBe(true);
    expect(shouldShowHeaderSearch("/admin/masters/abc/edit")).toBe(true);
    expect(shouldShowHeaderSearch("/admin/masters/abc/edit/")).toBe(true);
  });

  it("returns true on the catalog deck-detail route", () => {
    // Unlike the cardgroups/masters detail routes (whose card list lives at the
    // nested [id]/edit screen), the public catalog deck-detail route renders the
    // card list directly at /catalog/[id] and wires the header-takeover filter,
    // so the detail route itself is searchable.
    expect(shouldShowHeaderSearch("/catalog/abc")).toBe(true);
    expect(shouldShowHeaderSearch("/catalog/abc/")).toBe(true);
  });

  it("returns false on non-filterable routes", () => {
    expect(shouldShowHeaderSearch("/")).toBe(false);
    expect(shouldShowHeaderSearch("/learn/123")).toBe(false);
  });

  it("returns false on the detail (non-edit) routes", () => {
    // The cardgroups/masters detail routes carry no /edit segment, so they are
    // not searchable — only each list route and its [id]/edit card-list route
    // are. (The masters list route /admin/masters is itself searchable, so it is
    // not listed here. The catalog deck-detail route is the deliberate exception,
    // covered above, because its card list renders at /catalog/[id] directly.)
    expect(shouldShowHeaderSearch("/cardgroups/abc")).toBe(false);
    expect(shouldShowHeaderSearch("/admin/masters/abc")).toBe(false);
  });
});
