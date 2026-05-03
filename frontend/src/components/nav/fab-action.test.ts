// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveFabAction } from "./fab-action";

describe("resolveFabAction", () => {
  describe("creates a cardgroup on /cardgroups", () => {
    it("returns Add new cardgroup href for exact /cardgroups", () => {
      expect(resolveFabAction("/cardgroups")).toEqual({
        href: "/cardgroups/new",
        label: "Add new cardgroup",
      });
    });
  });

  describe("creates a card with cardgroup pre-selected on cardgroup detail/cards", () => {
    it.each([
      [
        "/cardgroups/abc-123",
        { href: "/cards/new?cardgroup=abc-123", label: "Add new card" },
      ],
      [
        "/cardgroups/abc-123/cards",
        { href: "/cards/new?cardgroup=abc-123", label: "Add new card" },
      ],
      [
        "/cardgroups/0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
        {
          href: "/cards/new?cardgroup=0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
          label: "Add new card",
        },
      ],
    ])("returns card href with cardgroup param for %s", (pathname, expected) => {
      expect(resolveFabAction(pathname)).toEqual(expected);
    });
  });

  describe("hides on cardgroups edit", () => {
    it.each([
      ["/cardgroups/abc-123/edit"],
      ["/cardgroups/abc-123/edit/"],
      ["/cardgroups/abc-123/edit/anything"],
    ])("returns null for %s", (pathname) => {
      expect(resolveFabAction(pathname)).toBeNull();
    });
  });

  describe("default to creating a card", () => {
    it.each([
      ["/"],
      ["/learn/abc-123"],
      ["/profile"],
      ["/login"],
    ])("returns generic /cards/new for %s", (pathname) => {
      expect(resolveFabAction(pathname)).toEqual({
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });
});
