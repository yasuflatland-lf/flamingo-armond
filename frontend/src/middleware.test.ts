import { unstable_doesMiddlewareMatch } from "next/experimental/testing/server";
import { describe, expect, it } from "vitest";
import { config } from "./middleware";

// Route Handlers that write their own auth cookies and the /api GraphQL proxy
// must stay outside the matcher: refreshed middleware cookies would race the
// handler's writes on the same response (see docs/frontend/auth-supabase.md).
describe("middleware matcher", () => {
  it.each([
    "/auth/callback",
    "/auth/verify-session",
    "/auth/verify-session?reason=x",
    "/api/graphql",
  ])("does not run for %s", (path) => {
    expect(unstable_doesMiddlewareMatch({ config, url: path })).toBe(false);
  });

  it.each(["/", "/login", "/login?reason=session_invalid", "/cardgroups"])(
    "runs for %s",
    (path) => {
      expect(unstable_doesMiddlewareMatch({ config, url: path })).toBe(true);
    },
  );
});
