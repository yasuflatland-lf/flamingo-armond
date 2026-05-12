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

  describe("creates a card with cardgroup pre-selected on cardgroups edit (integrated management screen)", () => {
    it.each([
      [
        "/cardgroups/abc-123/edit",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=abc-123",
          label: "Add new card",
          cardgroupId: "abc-123",
        },
      ],
      [
        "/cardgroups/abc-123/edit/",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=abc-123",
          label: "Add new card",
          cardgroupId: "abc-123",
        },
      ],
      [
        "/cardgroups/0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c/edit",
        {
          kind: "card-with-group",
          href: "/cards/new?cardgroup=0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
          label: "Add new card",
          cardgroupId: "0190f9f4-3ad8-7d52-b1d0-6f6b5f5a9c2c",
        },
      ],
    ])("returns card-with-group action for %s", (pathname, expected) => {
      expect(resolveFabAction(pathname)).toEqual(expected);
    });
  });

  describe("default to creating a card", () => {
    it.each([["/"], ["/profile"], ["/login"]])("returns generic /cards/new for %s", (pathname) => {
      expect(resolveFabAction(pathname)).toEqual({
        kind: "card",
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });

  describe("returns generic card action for /learn paths (GlobalFAB is hidden on /learn by its own guard)", () => {
    it.each([["/learn"], ["/learn/abc-123"], ["/learn/abc-123/"]])(
      "returns generic card action for %s",
      (pathname) => {
        expect(resolveFabAction(pathname)).toEqual({
          kind: "card",
          href: "/cards/new",
          label: "Add new card",
        });
      },
    );
  });

  describe("is shadowed externally by GlobalFAB's hidden-path guard for /cardgroups/new", () => {
    it("returns the generic card action for /cardgroups/new (no detail/cards regex matches it; GlobalFAB suppresses the FAB on this path anyway via its own hidden-path guard)", () => {
      expect(resolveFabAction("/cardgroups/new")).toEqual({
        kind: "card",
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });

  describe("URL-encodes captured cardgroup ids that contain special characters", () => {
    it("encodes & in id for /cardgroups/:id/edit branch — href is encoded, cardgroupId is raw", () => {
      const result = resolveFabAction("/cardgroups/abc&evil/edit");
      expect(result).toEqual({
        kind: "card-with-group",
        href: "/cards/new?cardgroup=abc%26evil",
        label: "Add new card",
        cardgroupId: "abc&evil",
      });
    });

    it("decodes percent-encoded segment in /cardgroups/:id/edit — usePathname delivers %26, cardgroupId is raw &", () => {
      // usePathname() returns percent-encoded pathnames. The function must decode
      // the captured segment before passing it to cardWithGroup so the downstream
      // encodeURIComponent encodes it exactly once.
      const result = resolveFabAction("/cardgroups/abc%26evil/edit");
      expect(result).toEqual({
        kind: "card-with-group",
        href: "/cards/new?cardgroup=abc%26evil",
        label: "Add new card",
        cardgroupId: "abc&evil",
      });
    });

    it("malformed percent-escape in /cardgroups/:id/edit falls through to generic card action", () => {
      // A URIError from decodeURIComponent must not propagate — it should produce
      // the same fallback as a non-matching path.
      expect(resolveFabAction("/cardgroups/abc%XX/edit")).toEqual({
        kind: "card",
        href: "/cards/new",
        label: "Add new card",
      });
    });
  });
});
