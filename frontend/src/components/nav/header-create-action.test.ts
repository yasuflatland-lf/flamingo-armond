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
    it.each([
      [
        "/cardgroups/abc/edit",
        {
          kind: "card-with-group",
          label: "Add new card",
          cardgroupId: "abc",
          href: "/cards/new?cardgroup=abc",
        },
      ],
      [
        "/cardgroups/abc/edit/",
        {
          kind: "card-with-group",
          label: "Add new card",
          cardgroupId: "abc",
          href: "/cards/new?cardgroup=abc",
        },
      ],
      [
        "/cardgroups/0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c/edit",
        {
          kind: "card-with-group",
          label: "Add new card",
          cardgroupId: "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
          href: "/cards/new?cardgroup=0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
        },
      ],
    ])("returns card-with-group for %s", (pathname, expected) => {
      expect(resolveHeaderCreateAction(pathname)).toEqual(expected);
    });

    it("decodes percent-encoded segment: a%2Fb -> cardgroupId a/b, href re-encodes", () => {
      expect(resolveHeaderCreateAction("/cardgroups/a%2Fb/edit")).toEqual({
        kind: "card-with-group",
        label: "Add new card",
        cardgroupId: "a/b",
        href: "/cards/new?cardgroup=a%2Fb",
      });
    });

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
