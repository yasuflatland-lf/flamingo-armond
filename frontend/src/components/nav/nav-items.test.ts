import { describe, expect, it } from "vitest";
import { ADMIN_NAV_ITEMS, CORE_NAV_ITEMS, resolveActiveItem } from "./nav-items";

describe("ADMIN_NAV_ITEMS", () => {
  it("includes a Masters entry pointing at /admin/masters", () => {
    const masters = ADMIN_NAV_ITEMS.find((i) => i.href === "/admin/masters");
    expect(masters).toBeDefined();
    expect(masters?.labelKey).toBe("masters");
  });

  it("keeps users and roles entries", () => {
    const hrefs = ADMIN_NAV_ITEMS.map((i) => i.href);
    expect(hrefs).toContain("/admin/users");
    expect(hrefs).toContain("/admin/roles");
  });
});

describe("CORE_NAV_ITEMS", () => {
  it("includes a Progress entry pointing at /stats", () => {
    const progress = CORE_NAV_ITEMS.find((i) => i.href === "/stats");
    expect(progress).toBeDefined();
    expect(progress?.labelKey).toBe("progress");
  });

  it("keeps the cardgroups and catalog entries", () => {
    const hrefs = CORE_NAV_ITEMS.map((i) => i.href);
    expect(hrefs).toContain("/cardgroups");
    expect(hrefs).toContain("/catalog");
  });
});

describe("resolveActiveItem", () => {
  it("resolves /stats and its sub-routes to progress", () => {
    expect(resolveActiveItem("/stats")).toBe("progress");
    expect(resolveActiveItem("/stats/anything")).toBe("progress");
  });

  it("keeps the cardgroups (incl. /learn) and catalog resolution", () => {
    expect(resolveActiveItem("/cardgroups")).toBe("cardgroups");
    expect(resolveActiveItem("/learn/abc")).toBe("cardgroups");
    expect(resolveActiveItem("/catalog")).toBe("catalog");
  });

  it("returns null for unrelated routes — the positive allowlist does not over-match", () => {
    expect(resolveActiveItem("/profile")).toBeNull();
    expect(resolveActiveItem("/admin/users")).toBeNull();
  });
});
