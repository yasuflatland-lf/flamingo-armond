import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

// Scenario: a user whose displayName is empty is redirected from / to /onboarding,
// fills in a display name, and submits. OnboardingForm pushes /onboarding/start
// (the first-deck chooser); because the e2e DB seeds no master decks, that route
// hits its empty-catalog fallback and redirects to /cardgroups/new?welcome=1,
// which is the URL this test waits for. Seeding a master deck would make the
// chooser render and stay on /onboarding/start instead.

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

    test("home redirects to /onboarding, form accepts display name, then lands on /cardgroups/new?welcome=1 via the /onboarding/start chooser fallback", async ({
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

      // 4. On success, the form pushes /onboarding/start (the first-deck chooser).
      //    With no master decks seeded, /onboarding/start hits its empty-catalog
      //    fallback and redirects to /cardgroups/new?welcome=1 — the URL we wait for.
      await page.waitForURL("**/cardgroups/new?welcome=1", { timeout: 15_000 });
    });
  });
