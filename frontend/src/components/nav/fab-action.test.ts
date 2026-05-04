// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveFabAction } from "./fab-action";

describe("resolveFabAction", () => {
  describe("creates a cardgroup on /cardgroups", () => {
    it("returns Add new cardgroup href for exact /cardgroups", () => {
      expect(resolveFabAction("/cardgroups")).toEqual({
        kind: "cardgroup",
        href: "/cardgroups/new",
        label: "Add new cardgroup",
      });
    });
  });

  describe("creates a card with cardgroup pre-selected on cardgroup detail/cards", () => {
    it.each([
      [
        "/cardgroups/abc-123",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=abc-123",
          label: "Add new card",
          cardgroupId: "abc-123",
        },
      ],
      [
        "/cardgroups/abc-123/cards",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=abc-123",
          label: "Add new card",
          cardgroupId: "abc-123",
        },
      ],
      [
        "/cardgroups/0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
          label: "Add new card",
          cardgroupId: "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
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
      ["/profile"],
      ["/login"],
    ])("returns generic /cards/new for %s", (pathname) => {
      expect(resolveFabAction(pathname)).toEqual({
        kind: "card",
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });

  describe("creates a card with cardgroup pre-selected and return param on /learn/:id", () => {
    it("returns card-with-group action with return param for /learn/abc-123", () => {
      expect(resolveFabAction("/learn/abc-123")).toEqual({
        kind: "card-with-group",
        href: "/cards/new?cardgroup=abc-123&return=/learn/abc-123",
        label: "Add new card",
        cardgroupId: "abc-123",
      });
    });

    it("falls through to generic card action for /learn (no id segment)", () => {
      expect(resolveFabAction("/learn")).toEqual({
        kind: "card",
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });

  describe("is shadowed externally by GlobalFAB's hidden-path guard for /cardgroups/new", () => {
    it("returns a card-with-group action (the literal 'new' is treated as a cardgroup id; GlobalFAB suppresses the FAB on this path via its own hidden-path guard)", () => {
      expect(resolveFabAction("/cardgroups/new")).toEqual({
        kind: "card-with-group",
        href: "/cards/new?cardgroup=new",
        label: "Add new card",
        cardgroupId: "new",
      });
    });
  });
});
