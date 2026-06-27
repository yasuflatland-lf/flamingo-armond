import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

// Scenario: a brand-new user (zero cardgroups, no last_viewed) is funnelled by
// the HomePage chain to /onboarding/start (the first-deck chooser). Because the
// e2e DB seeds no master decks, that route hits its empty-catalog fallback and
// redirects to /cardgroups/new?welcome=1, where the user creates their first
// cardgroup; the nav-header "+" button then opens the inline "Add card"
// FormSheet on the cardgroup edit page (no navigation to /cards/new).

const runId = randomUUID().slice(0, 8);
const newcomer = {
  email: `new-onboarding-${runId}@example.test`,
  password: "e2e-password",
};
const cardgroupName = `E2E onboarding ${runId}`;
const UUID_RE = /^[0-9a-f-]{36}$/;

test.describe
  .serial("new user onboarding", () => {
    // The nav-header "+" button is rendered inside the `md:hidden` mobile
    // header and is not present at >= md viewports. This scenario exercises
    // the mobile path, so shrink the viewport below the md breakpoint (768px).
    // We only override viewport here — spreading a full devices[...] descriptor
    // includes defaultBrowserType, which Playwright forbids inside a describe
    // group ("forces a new worker") and would also require a matching project
    // entry, neither of which we want here.
    test.use({ viewport: { width: 390, height: 844 } });

    test.beforeAll(async () => {
      // Seed only the auth user + role; deliberately NO cardgroup so the home
      // RSC routes the deckless-onboarded user to /onboarding/start on first
      // login (which falls back to /cardgroups/new?welcome=1 — see file header).
      await seedUser({
        email: newcomer.email,
        password: newcomer.password,
        role: "general",
        displayName: "E2E New User",
      });
    });

    test("welcome page → create cardgroup → nav-header '+' opens inline Add card sheet", async ({
      context,
      page,
    }) => {
      await loginAs(context, newcomer);

      // 1. Home routes the deckless-onboarded user to /onboarding/start, which
      //    (no master decks seeded) falls back to /cardgroups/new?welcome=1.
      await page.goto("/");
      await page.waitForURL("**/cardgroups/new?welcome=1", { timeout: 10_000 });
      // Use the existing id="welcome-heading" to avoid locale-dependent text matching
      // (Playwright runs with ja-JP locale so translated text won't match English regex).
      await expect(page.locator("#welcome-heading")).toBeVisible();

      // 2. Submit the create-cardgroup form. onCompleted pushes /cardgroups/<newId>,
      // which redirects server-side to /cardgroups/<newId>/edit. Wait for the
      // /edit form so we can capture the new id from the URL.
      // Use name attribute to avoid locale-dependent label text (Playwright runs ja-JP).
      await page.locator('input[name="name"]').fill(cardgroupName);
      // Use data-testid to avoid locale-dependent button text (Playwright runs ja-JP).
      await page.getByTestId("cardgroup-form-submit").click();
      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}\/edit$/, { timeout: 15_000 });
      const newCardgroupId = page.url().split("/").slice(-2, -1)[0] ?? "";
      expect(newCardgroupId).toMatch(UUID_RE);
      await expect(page.getByRole("heading", { name: cardgroupName })).toBeVisible();

      // 3. The nav-header "+" on /cardgroups/<id>/edit opens an Add menu
      // (Add card / Batch import / Merge). Open it via its locale-independent
      // testid, then choose "Add card", which dispatches the cancelable
      // flamingo:add-card event the in-page sheet claims — opening the "Add
      // card" FormSheet inline with no navigation to /cards/new.
      await page.getByTestId("header-add-menu-trigger").click();
      await page.getByTestId("header-add-card").click();

      // 4. FormSheet opens inline. On mobile (390x844, below md=768) FormSheet
      // renders as a vaul Drawer. Verify the form is interactive via the Front
      // field — a stronger locale-independent signal than the translated sheet title.
      // Use name attribute to avoid locale-dependent label text (Playwright runs ja-JP).
      await expect(page.locator('input[name="front"]')).toBeVisible();

      // URL must NOT have navigated to /cards/new — the inline-sheet path is
      // the entire point of the FormSheet migration. Stay on /edit.
      expect(page.url()).toMatch(/\/cardgroups\/[0-9a-f-]{36}\/edit$/);
    });
  });
