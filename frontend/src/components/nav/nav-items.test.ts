import { describe, expect, it } from "vitest";
import { ADMIN_NAV_ITEMS } from "./nav-items";

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
