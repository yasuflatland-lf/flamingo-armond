import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { deleteAuthUser, loginAs, seedUser } from "./_auth";

// Scenario: the account is deleted while the browser still holds a signed,
// unexpired access token. `/` must end on /login instead of bouncing / <-> /login
// until the browser gives up with ERR_TOO_MANY_REDIRECTS.

const runId = randomUUID().slice(0, 8);
const learner = {
  email: `deleted-session-${runId}@example.test`,
  password: "e2e-password",
};

test("deleted account with a live session cookie lands on /login", async ({ context, page }) => {
  const user = await seedUser({
    email: learner.email,
    password: learner.password,
    role: "general",
    displayName: "E2E Deleted Learner",
  });
  await loginAs(context, learner);

  await page.goto("/");
  await page.waitForURL(/\/(cardgroups|onboarding\/start)(\?|$)/, { timeout: 10_000 });

  await deleteAuthUser(user.id);

  // A redirect loop rejects page.goto with net::ERR_TOO_MANY_REDIRECTS.
  await page.goto("/");

  // Whether the local stack reproduces the loop depends on its JWT signing
  // key type, so only termination on /login is asserted, not `reason=`.
  // Not a one-shot page.url() read: the root loading.tsx streams `/` first, so
  // goto resolves before the RSC redirect moves the client to /login.
  await page.waitForURL(/\/login(\?|$)/, { timeout: 10_000 });
  await expect(page.getByTestId("login-google-button")).toBeVisible();
});
