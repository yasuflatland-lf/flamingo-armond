import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { assertNoPublishedMasters, loginAs, seedUser } from "./_auth";

// Scenario: a user whose displayName is empty is redirected to /onboarding, fills
// in a display name, and submits. OnboardingForm pushes /onboarding/start (the
// first-deck chooser); because the e2e DB holds no PUBLISHED master deck — the only
// ones any spec seeds are DRAFT — that route hits its empty-catalog fallback and
// redirects to /cardgroups/new?welcome=1, which is the URL this test waits for.
// Publishing a deck with cards would make the chooser render and stay on
// /onboarding/start instead; assertNoPublishedMasters() in beforeAll pins that.
//
// The first hop is now owned by the MIDDLEWARE display-name gate, not by the home
// RSC: the gate redirects every non-exempt path to /onboarding while displayName is
// empty, so this user reaches /onboarding from any URL, and / never gets as far as
// running its own isUserOnboarded branch. /onboarding and /onboarding/start are
// exempt from the gate, so the form and the chooser stay reachable in that state.
// /cardgroups/new is gated, and is reached only because the submit has by then made
// displayName non-empty.

const runId = randomUUID().slice(0, 8);
const onboarder = {
  email: `display-name-onboarding-${runId}@example.test`,
  password: "e2e-password",
};

test.describe
  .serial("display name onboarding redirect", () => {
    test.beforeAll(async () => {
      await assertNoPublishedMasters();
      // Seed with displayName: "" so isUserOnboarded() returns false and the
      // middleware gate redirects to /onboarding instead of letting the home RSC
      // route the user on to /cardgroups.
      await seedUser({
        email: onboarder.email,
        password: onboarder.password,
        role: "general",
        displayName: "",
      });
    });

    test("the display-name gate redirects to /onboarding, the form accepts a display name, then lands on /cardgroups/new?welcome=1 via the /onboarding/start chooser fallback", async ({
      context,
      page,
    }) => {
      await loginAs(context, onboarder);

      // 1. The middleware gate detects the empty displayName and redirects to
      //    /onboarding before / renders at all.
      await page.goto("/");
      await page.waitForURL("**/onboarding", { timeout: 10_000 });

      // 1b. The same gate covers deep links, which the home-RSC branch never did:
      //     a direct /cardgroups hit lands on /onboarding too.
      await page.goto("/cardgroups");
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
