import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

// Scenario: a brand-new user (zero cardgroups, no last_viewed) is funnelled
// into onboarding (/cardgroups/new?welcome=1), creates their first cardgroup,
// and the nav-header "+" button opens the inline "Add card" FormSheet on the
// cardgroup edit page (no navigation to /cards/new).

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
      // RSC routes to /cardgroups/new?welcome=1 on first login.
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

      // 1. Home redirects to onboarding because user has no cardgroups.
      await page.goto("/");
      await page.waitForURL("**/cardgroups/new?welcome=1", { timeout: 10_000 });
      await expect(
        page.getByRole("heading", { name: /Welcome!.*create your first cardgroup/i }),
      ).toBeVisible();

      // 2. Submit the create-cardgroup form. onCompleted pushes /cardgroups/<newId>,
      // which redirects server-side to /cardgroups/<newId>/edit. Wait for the
      // /edit form so we can capture the new id from the URL.
      await page.getByLabel("Name").fill(cardgroupName);
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}\/edit$/, { timeout: 15_000 });
      const newCardgroupId = page.url().split("/").slice(-2, -1)[0] ?? "";
      expect(newCardgroupId).toMatch(UUID_RE);
      await expect(page.getByRole("heading", { name: cardgroupName })).toBeVisible();

      // 3. Click the nav-header "+" button. On /cardgroups/<id>/edit its
      // aria-label is "Add new card"; clicking it opens the "Add card"
      // FormSheet inline — no navigation to /cards/new.
      const addCardButton = page.getByRole("button", { name: "Add new card" });
      await expect(addCardButton).toBeVisible();
      await addCardButton.click();

      // 4. FormSheet opens inline. On mobile (390x844, below md=768) FormSheet
      // renders as a vaul Drawer with DrawerTitle "Add card".
      await expect(page.getByRole("heading", { name: "Add card" })).toBeVisible();

      // CardForm in mode="create" renders the Front field; its visibility is a
      // stronger signal than the heading alone that the sheet content mounted.
      await expect(page.getByLabel("Front")).toBeVisible();

      // URL must NOT have navigated to /cards/new — the inline-sheet path is
      // the entire point of the FormSheet migration. Stay on /edit.
      expect(page.url()).toMatch(/\/cardgroups\/[0-9a-f-]{36}\/edit$/);
    });
  });
