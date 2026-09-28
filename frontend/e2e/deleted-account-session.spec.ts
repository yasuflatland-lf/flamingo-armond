import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { deleteAuthUser, loginAs, seedUser } from "./_auth";

// Scenario: the account is deleted while the browser still holds a signed,
// unexpired access token. `/` must end on /login instead of bouncing / <-> /login.

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

  // Not relying on goto rejecting with ERR_TOO_MANY_REDIRECTS: streaming makes a loop
  // client-side, so goto resolves either way. The LoginButton assertion below catches
  // it, because a looping /login redirects before rendering the button.
  await page.goto("/");

  // Whether the local stack reproduces the loop depends on its JWT signing
  // key type, so only termination on /login is asserted, not `reason=`.
  // Not a one-shot page.url() read: wait for the RSC redirect to reach /login.
  await page.waitForURL(/\/login(\?|$)/, { timeout: 10_000 });
  await expect(page.getByTestId("login-google-button")).toBeVisible();
});
