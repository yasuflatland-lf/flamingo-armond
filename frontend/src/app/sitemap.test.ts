import { describe, expect, test } from "vitest";
import sitemap from "@/app/sitemap";

describe("sitemap()", () => {
  const entries = sitemap();

  test("contains exactly two entries: / and /login", () => {
    const paths = entries.map((e) => new URL(e.url).pathname);
    expect(paths).toHaveLength(2);
    expect(paths).toContain("/");
    expect(paths).toContain("/login");
  });

  test("does not include any dynamic segment (no [ or ] in URLs)", () => {
    for (const entry of entries) {
      expect(entry.url).not.toContain("[");
      expect(entry.url).not.toContain("]");
    }
  });

  test("does not include any private route prefix", () => {
    const privatePatterns = ["/admin", "/profile", "/learn", "/cardgroups", "/api", "/auth"];
    for (const entry of entries) {
      const { pathname } = new URL(entry.url);
      for (const prefix of privatePatterns) {
        expect(pathname).not.toMatch(new RegExp(`^${prefix}`));
      }
    }
  });

  test("all URLs use the site origin and are absolute", () => {
    for (const entry of entries) {
      expect(entry.url).toMatch(/^https?:\/\//);
    }
  });

  test("each entry carries lastModified, changeFrequency, and priority", () => {
    for (const entry of entries) {
      expect(entry.lastModified).toBeInstanceOf(Date);
      expect(entry.changeFrequency).toBeDefined();
      expect(entry.priority).toBeTypeOf("number");
    }
  });
});
