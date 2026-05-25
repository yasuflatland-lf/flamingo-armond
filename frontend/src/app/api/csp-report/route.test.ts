import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";

function makeRequest(contentType: string, body: unknown) {
  return new Request("http://localhost/api/csp-report", {
    method: "POST",
    headers: {
      "content-type": contentType,
    },
    body: typeof body === "string" ? body : JSON.stringify(body),
  });
}

describe("POST /api/csp-report", () => {
  let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleWarnSpy.mockRestore();
    vi.clearAllMocks();
  });

  it("accepts application/csp-report and logs a normalized report", async () => {
    const response = await POST(
      makeRequest("application/csp-report", {
        "csp-report": {
          "document-uri": "https://example.com/page",
          "violated-directive": "script-src-elem",
          "effective-directive": "script-src-elem",
          "original-policy": "default-src 'self'",
          "blocked-uri": "inline",
          "line-number": 12,
          "status-code": 200,
          disposition: "report",
        },
      }),
    );

    expect(response.status).toBe(204);
    expect(consoleWarnSpy).toHaveBeenCalledWith("[csp-report] received report", {
      contentType: "application/csp-report",
      report: {
        type: "csp-violation",
        url: "https://example.com/page",
        userAgent: null,
        body: {
          blockedUrl: "inline",
          disposition: "report",
          documentUrl: "https://example.com/page",
          effectiveDirective: "script-src-elem",
          lineNumber: 12,
          originalPolicy: "default-src 'self'",
          statusCode: 200,
          violatedDirective: "script-src-elem",
        },
      },
    });
  });

  it("accepts application/reports+json and logs a normalized report", async () => {
    const response = await POST(
      makeRequest("application/reports+json", [
        {
          age: 0,
          type: "csp-violation",
          url: "https://example.com/page",
          user_agent: "Mozilla/5.0",
          body: {
            blockedURL: "inline",
            disposition: "report",
            documentURL: "https://example.com/page",
            effectiveDirective: "script-src-elem",
            originalPolicy: "default-src 'self'",
            statusCode: 200,
            violatedDirective: "script-src-elem",
          },
        },
      ]),
    );

    expect(response.status).toBe(204);
    expect(consoleWarnSpy).toHaveBeenCalledWith("[csp-report] received report", {
      contentType: "application/reports+json",
      report: {
        type: "csp-violation",
        age: 0,
        url: "https://example.com/page",
        userAgent: "Mozilla/5.0",
        body: {
          blockedUrl: "inline",
          disposition: "report",
          documentUrl: "https://example.com/page",
          effectiveDirective: "script-src-elem",
          originalPolicy: "default-src 'self'",
          statusCode: 200,
          violatedDirective: "script-src-elem",
        },
      },
    });
  });

  it("accepts JSON-ish application/json payloads", async () => {
    const response = await POST(
      makeRequest("application/json", {
        "csp-report": {
          "document-uri": "https://example.com/page",
          "violated-directive": "script-src-elem",
          "blocked-uri": "inline",
        },
      }),
    );

    expect(response.status).toBe(204);
    expect(consoleWarnSpy).toHaveBeenCalledWith("[csp-report] received report", {
      contentType: "application/json",
      report: {
        type: "csp-violation",
        url: "https://example.com/page",
        userAgent: null,
        body: {
          blockedUrl: "inline",
          documentUrl: "https://example.com/page",
          violatedDirective: "script-src-elem",
        },
      },
    });
  });

  it("returns 204 and logs a warning for malformed payloads", async () => {
    const response = await POST(
      makeRequest("application/csp-report", "{not-json"),
    );

    expect(response.status).toBe(204);
    expect(consoleWarnSpy).toHaveBeenCalledWith("[csp-report] invalid report payload", {
      contentType: "application/csp-report",
      reason: "invalid_json",
    });
  });
});
