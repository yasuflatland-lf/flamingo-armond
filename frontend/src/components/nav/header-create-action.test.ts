// @vitest-environment node
import { describe, expect, it } from "vitest";
import { resolveHeaderCreateAction } from "./header-create-action";

describe("resolveHeaderCreateAction", () => {
  describe("/cardgroups route", () => {
    it("returns cardgroup action for exact /cardgroups", () => {
      expect(resolveHeaderCreateAction("/cardgroups")).toEqual({
        kind: "cardgroup",
        label: "Add new cardgroup",
        href: "/cardgroups/new",
      });
    });
  });

  describe("/cardgroups/:id/edit route", () => {
    it("returns null for malformed percent-escape in /cardgroups/:id/edit", () => {
      expect(resolveHeaderCreateAction("/cardgroups/%ZZ/edit")).toBeNull();
    });

    it("returns null for /cardgroups/abc (no /edit)", () => {
      expect(resolveHeaderCreateAction("/cardgroups/abc")).toBeNull();
    });

    it("returns null for /cardgroups/abc/cards", () => {
      expect(resolveHeaderCreateAction("/cardgroups/abc/cards")).toBeNull();
    });
  });

  describe("/learn/:id route", () => {
    it("returns card-with-group with return param for /learn/xyz", () => {
      expect(resolveHeaderCreateAction("/learn/xyz")).toEqual({
        kind: "card-with-group",
        label: "Add new card",
        cardgroupId: "xyz",
        href: "/cards/new?cardgroup=xyz&return=/learn/xyz",
      });
    });

    it("returns card-with-group with trailing slash /learn/xyz/", () => {
      expect(resolveHeaderCreateAction("/learn/xyz/")).toEqual({
        kind: "card-with-group",
        label: "Add new card",
        cardgroupId: "xyz",
        href: "/cards/new?cardgroup=xyz&return=/learn/xyz",
      });
    });

    it("decodes a%2Fb in /learn/:id and re-encodes in href", () => {
      expect(resolveHeaderCreateAction("/learn/a%2Fb")).toEqual({
        kind: "card-with-group",
        label: "Add new card",
        cardgroupId: "a/b",
        href: "/cards/new?cardgroup=a%2Fb&return=/learn/a%2Fb",
      });
    });

    it("returns null for malformed percent-escape in /learn/:id", () => {
      expect(resolveHeaderCreateAction("/learn/%ZZ")).toBeNull();
    });

    it("returns null for bare /learn with no id segment", () => {
      expect(resolveHeaderCreateAction("/learn")).toBeNull();
    });
  });

  describe("/admin/roles route", () => {
    it("returns role action for exact /admin/roles", () => {
      expect(resolveHeaderCreateAction("/admin/roles")).toEqual({
        kind: "role",
        label: "Add new role",
      });
    });
  });

  describe("/admin/masters route", () => {
    it("returns master action for exact /admin/masters", () => {
      expect(resolveHeaderCreateAction("/admin/masters")).toEqual({
        kind: "master",
        label: "Add new master",
      });
    });
  });

  describe("/admin/masters/:id/edit route", () => {
    it("returns null for malformed percent-escape in /admin/masters/:id/edit", () => {
      expect(resolveHeaderCreateAction("/admin/masters/%ZZ/edit")).toBeNull();
    });

    it("returns null for /admin/masters/m-1 (no /edit)", () => {
      expect(resolveHeaderCreateAction("/admin/masters/m-1")).toBeNull();
    });
  });

  describe("/admin/masters/:id/edit -> deck-add-menu (master)", () => {
    it("returns a master deck-add-menu", () => {
      expect(resolveHeaderCreateAction("/admin/masters/m-1/edit")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "master", masterId: "m-1" },
      });
    });
    it("decodes a%2Fb -> masterId a/b", () => {
      expect(resolveHeaderCreateAction("/admin/masters/a%2Fb/edit")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "master", masterId: "a/b" },
      });
    });
    it("returns null for malformed escape", () => {
      expect(resolveHeaderCreateAction("/admin/masters/%ZZ/edit")).toBeNull();
    });
    it("matches trailing slash /admin/masters/m-1/edit/", () => {
      expect(resolveHeaderCreateAction("/admin/masters/m-1/edit/")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "master", masterId: "m-1" },
      });
    });
  });

  describe("/cardgroups/:id/edit -> deck-add-menu (cardgroup)", () => {
    it("returns a cardgroup deck-add-menu with encoded addCardHref", () => {
      expect(resolveHeaderCreateAction("/cardgroups/abc/edit")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "cardgroup", cardgroupId: "abc", addCardHref: "/cards/new?cardgroup=abc" },
      });
    });
    it("decodes a%2Fb and re-encodes in addCardHref", () => {
      expect(resolveHeaderCreateAction("/cardgroups/a%2Fb/edit")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "cardgroup", cardgroupId: "a/b", addCardHref: "/cards/new?cardgroup=a%2Fb" },
      });
    });
    it("matches trailing slash /cardgroups/abc/edit/", () => {
      expect(resolveHeaderCreateAction("/cardgroups/abc/edit/")).toEqual({
        kind: "deck-add-menu",
        deck: { kind: "cardgroup", cardgroupId: "abc", addCardHref: "/cards/new?cardgroup=abc" },
      });
    });
  });

  describe("routes that return null", () => {
    it.each([
      ["/"],
      ["/cards/new"],
      ["/profile"],
      ["/admin"],
      ["/admin/users"],
      ["/cardgroups/new"],
      ["/login"],
    ])("returns null for %s", (pathname) => {
      expect(resolveHeaderCreateAction(pathname)).toBeNull();
    });
  });
});
