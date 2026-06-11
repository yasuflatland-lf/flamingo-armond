import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

// Scenario: a user whose displayName is empty is redirected from / to /onboarding,
// fills in a display name, submits, and lands on /cardgroups/new?welcome=1.

const runId = randomUUID().slice(0, 8);
const onboarder = {
  email: `display-name-onboarding-${runId}@example.test`,
  password: "e2e-password",
};

test.describe
  .serial("display name onboarding redirect", () => {
    test.beforeAll(async () => {
      // Seed with displayName: "" so isUserOnboarded() returns false and
      // the home RSC redirects to /onboarding instead of /cardgroups.
      await seedUser({
        email: onboarder.email,
        password: onboarder.password,
        role: "general",
        displayName: "",
      });
    });

    test("home redirects to /onboarding, form accepts display name, then lands on /cardgroups/new?welcome=1", async ({
      context,
      page,
    }) => {
      await loginAs(context, onboarder);

      // 1. Home RSC detects empty displayName and redirects to /onboarding.
      await page.goto("/");
      await page.waitForURL("**/onboarding", { timeout: 10_000 });

      // 2. Onboarding form is visible. The suite runs in the ja-JP locale, so
      // target the display-name input and submit button by locale-independent
      // attributes (input id / data-testid) rather than translated copy.
      const displayNameInput = page.locator("#displayName");
      await expect(displayNameInput).toBeVisible();

      // 3. Fill in the display name and submit.
      await displayNameInput.fill("E2E Onboarder");
      await page.getByTestId("onboarding-submit").click();

      // 4. On success, the form's onCompleted callback pushes /cardgroups/new?welcome=1.
      await page.waitForURL("**/cardgroups/new?welcome=1", { timeout: 15_000 });
    });
  });
