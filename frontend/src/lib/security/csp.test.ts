import { describe, expect, it } from "vitest";

import { buildHtmlCsp, serializeCsp } from "./csp";

describe("serializeCsp", () => {
  it("serializes directives in a stable order and removes duplicate or empty sources", () => {
    const policy = serializeCsp({
      "img-src": ["'self'", "", "data:", "data:", "  ", "blob:"],
      "default-src": ["'self'"],
      "style-src": ["'self'", "'unsafe-inline'", "'self'", undefined],
    });

    expect(policy).toBe(
      "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'",
    );
  });
});

describe("buildHtmlCsp", () => {
  it("injects the nonce and includes the HTML policy directives the app needs", () => {
    const policy = buildHtmlCsp({
      nonce: "abc123",
      supabaseUrl: "https://project-ref.supabase.co",
    });

    expect(policy).toBe(
      [
        "default-src 'self'",
        "base-uri 'self'",
        "object-src 'none'",
        "frame-ancestors 'none'",
        "form-action 'self'",
        "img-src 'self' data: blob: https://lh3.googleusercontent.com",
        "style-src 'self' 'unsafe-inline'",
        "script-src 'self' 'nonce-abc123'",
        "connect-src 'self' https://project-ref.supabase.co wss://project-ref.supabase.co https://vitals.vercel-insights.com",
        "report-uri /api/csp-report",
        "report-to csp-endpoint",
      ].join("; "),
    );
  });

  it("keeps the same-origin /api/graphql path covered by connect-src 'self' and derives the Supabase realtime origin", () => {
    const policy = buildHtmlCsp({
      nonce: "nonce-value",
      supabaseUrl: "https://foo.supabase.co",
      reportTo: "csp-endpoint",
      reportUri: "/api/csp-report",
    });

    expect(policy).toContain(
      "connect-src 'self' https://foo.supabase.co wss://foo.supabase.co https://vitals.vercel-insights.com",
    );
  });

  it("includes reporting directives for the reporting-endpoints rollout", () => {
    const policy = buildHtmlCsp({
      nonce: "nonce-value",
      supabaseUrl: "https://foo.supabase.co",
      reportTo: "csp-endpoint",
      reportUri: "/api/csp-report",
    });

    expect(policy).toContain("report-uri /api/csp-report");
    expect(policy).toContain("report-to csp-endpoint");
  });

  it("throws when nonce is empty", () => {
    expect(() =>
      buildHtmlCsp({ nonce: "", supabaseUrl: "https://project-ref.supabase.co" }),
    ).toThrow("nonce is required");
  });

  it("throws when nonce is whitespace-only", () => {
    expect(() =>
      buildHtmlCsp({ nonce: "   ", supabaseUrl: "https://project-ref.supabase.co" }),
    ).toThrow("nonce is required");
  });

  it("converts http:// supabaseUrl to ws:// in connect-src", () => {
    const policy = buildHtmlCsp({
      nonce: "abc123",
      supabaseUrl: "http://127.0.0.1:54321",
    });

    expect(policy).toContain("http://127.0.0.1:54321 ws://127.0.0.1:54321");
  });
});
