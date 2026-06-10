import { describe, expect, test } from "vitest";
import robots from "@/app/robots";

describe("robots()", () => {
  const result = robots();

  test("disallows /admin/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/admin/");
  });

  test("disallows /api/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/api/");
  });

  test("disallows /profile/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/profile/");
  });

  test("disallows /learn/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/learn/");
  });

  test("disallows /cardgroups/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/cardgroups/");
  });

  test("disallows /auth/ from crawlers", () => {
    const { disallow } = result.rules as { disallow: string[] };
    expect(disallow).toContain("/auth/");
  });

  test("allows the root path", () => {
    const { allow } = result.rules as { allow: string };
    expect(allow).toBe("/");
  });

  test("applies rules to all user agents", () => {
    const { userAgent } = result.rules as { userAgent: string };
    expect(userAgent).toBe("*");
  });

  test("sitemap URL points to /sitemap.xml under the site origin", () => {
    expect(result.sitemap).toMatch(/\/sitemap\.xml$/);
    expect(result.sitemap).toMatch(/^https?:\/\//);
  });
});
