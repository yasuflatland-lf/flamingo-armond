import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

// Scenario: a brand-new user (zero cardgroups, no last_viewed) is funnelled
// into onboarding (/cardgroups/new?welcome=1), creates their first cardgroup,
// and the global FAB takes them to /cards/new where the chip is pre-selected
// to that newly-created cardgroup.

const runId = randomUUID().slice(0, 8);
const newcomer = {
  email: `new-onboarding-${runId}@example.test`,
  password: "e2e-password",
};
const cardgroupName = `E2E onboarding ${runId}`;
const UUID_RE = /^[0-9a-f-]{36}$/;

test.describe
  .serial("new user onboarding", () => {
    // The global FAB is intentionally hidden at >= md (`md:hidden` in
    // GlobalFAB) because desktop users get the same affordance via the
    // header "Add card" link. This scenario is specifically the FAB path,
    // so shrink the viewport below the md breakpoint (768px). We only
    // override viewport here — spreading a full devices[...] descriptor
    // includes defaultBrowserType, which Playwright forbids inside a
    // describe group ("forces a new worker") and would also require a
    // matching project entry, neither of which we want here.
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

    test("welcome page → create cardgroup → FAB lands on /cards/new with chip pre-selected", async ({
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
      await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();

      // 2. Submit the create-cardgroup form. onCompleted pushes /cardgroups/<newId>;
      // wait for the navigation rather than a fixed URL so we can capture the new id.
      await page.getByLabel("Name").fill(cardgroupName);
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}$/, { timeout: 15_000 });
      const newCardgroupId = page.url().split("/").pop() ?? "";
      expect(newCardgroupId).toMatch(UUID_RE);
      await expect(page.getByRole("heading", { name: cardgroupName })).toBeVisible();

      // 3. Click the global "+ Card" FAB. On a /cardgroups/<id> page,
      // resolveFabAction returns kind: "card-with-group" so the FAB href is
      // /cards/new?cardgroup=<id> — no picker dialog, the chip is pre-selected
      // from the URL. The FAB renders at the root layout level and is hidden on
      // /learn/*, /admin/*, /cards/new, /cardgroups/new; /cardgroups/<id> is
      // not on the hidden list so the FAB must be present here.
      const fab = page.getByRole("button", { name: "Add new card" });
      await expect(fab).toBeVisible();
      await fab.click();

      // 4. URL carries ?cardgroup=<id> from the moment we land — no picker
      // round-trip needed because the cardgroup id was propagated via the FAB
      // href.
      await page.waitForURL(`**/cards/new?cardgroup=${newCardgroupId}`, {
        timeout: 10_000,
      });

      await expect(page.getByRole("heading", { name: "New card" })).toBeVisible();

      // The CardgroupChip's aria-label is "Change cardgroup (currently \"<name>\")"
      // when populated; the "Select cardgroup" placeholder means none selected.
      await expect(
        page.getByRole("button", {
          name: `Change cardgroup (currently "${cardgroupName}")`,
        }),
      ).toBeVisible();

      // CardForm renders only when currentId != null — Front being interactable
      // is a stronger signal than chip text alone that the resolved cardgroupId
      // reached CardsNewClient.
      await expect(page.getByLabel("Front")).toBeVisible();
    });
  });
