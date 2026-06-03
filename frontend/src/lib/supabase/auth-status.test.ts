import { describe, expect, it } from "vitest";
import {
  AUTH_STATUS_HEADER,
  IDENTITY_HEADERS,
  USER_EMAIL_HEADER,
  USER_IS_ADMIN_HEADER,
  readAuthContext,
} from "./auth-status";

function headersWith(entries: Record<string, string>): Headers {
  const h = new Headers();
  for (const [k, v] of Object.entries(entries)) h.set(k, v);
  return h;
}

describe("readAuthContext", () => {
  it("reads an authenticated admin context", () => {
    const ctx = readAuthContext(
      headersWith({
        [AUTH_STATUS_HEADER]: "authenticated",
        [USER_EMAIL_HEADER]: "a@b.c",
        [USER_IS_ADMIN_HEADER]: "true",
      }),
    );
    expect(ctx).toEqual({ status: "authenticated", email: "a@b.c", isAdmin: true });
  });

  it("maps an empty email header to null and missing admin to false", () => {
    const ctx = readAuthContext(
      headersWith({ [AUTH_STATUS_HEADER]: "anonymous", [USER_EMAIL_HEADER]: "" }),
    );
    expect(ctx).toEqual({ status: "anonymous", email: null, isAdmin: false });
  });

  it("falls back to anonymous for a missing or invalid status header", () => {
    expect(readAuthContext(new Headers()).status).toBe("anonymous");
    expect(readAuthContext(headersWith({ [AUTH_STATUS_HEADER]: "garbage" })).status).toBe("anonymous");
  });

  it("exposes the three identity header names for stripping", () => {
    expect(IDENTITY_HEADERS).toEqual([AUTH_STATUS_HEADER, USER_EMAIL_HEADER, USER_IS_ADMIN_HEADER]);
  });
});
