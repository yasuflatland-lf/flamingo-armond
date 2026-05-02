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

test.describe
  .serial("new user onboarding", () => {
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

    test("welcome page → create cardgroup → FAB → picker → chip pre-select", async ({
      context,
      page,
    }) => {
      await loginAs(context, newcomer);

      // ── 1. Home redirects to onboarding because user has no cardgroups. ───
      await page.goto("/");
      await page.waitForURL("**/cardgroups/new?welcome=1", { timeout: 10_000 });
      await expect(
        page.getByRole("heading", { name: /Welcome!.*create your first cardgroup/i }),
      ).toBeVisible();
      await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();

      // ── 2. Submit the create-cardgroup form. ──────────────────────────────
      await page.getByLabel("Name").fill(cardgroupName);
      await page.getByRole("button", { name: "Create" }).click();

      // The mutation's onCompleted pushes /cardgroups/<newId>; wait for the
      // navigation rather than asserting on the URL string directly so we
      // capture the new id.
      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}$/, { timeout: 15_000 });
      const cardgroupDetailUrl = page.url();
      const newCardgroupId = cardgroupDetailUrl.split("/").pop() ?? "";
      expect(newCardgroupId).toMatch(/^[0-9a-f-]{36}$/);
      await expect(page.getByRole("heading", { name: cardgroupName })).toBeVisible();

      // ── 3. Click the global "+ Card" FAB to open /cards/new. ─────────────
      // The FAB renders at the root layout level and is hidden on a few paths
      // (/learn/*, /admin/*, /cards/new, /cardgroups/new); /cardgroups/<id> is
      // not on the hidden list so the FAB must be present here.
      const fab = page.getByRole("button", { name: "Add new card" });
      await expect(fab).toBeVisible();
      await fab.click();

      // The FAB at the root layout is invoked without lastViewedCardgroupId, so
      // it navigates to bare /cards/new. The /cards/new RSC then resolves the
      // cardgroup: ?cardgroup= is empty, me.lastViewedCardgroup is still null
      // (the user has not visited /learn yet), and myCardgroups has exactly one
      // entry → forcePickerOpen=true so the picker auto-opens with the
      // chip in its undetermined "Select cardgroup" state.
      await page.waitForURL("**/cards/new", { timeout: 10_000 });

      // ── 4. The cardgroup picker auto-opens — choose the new cardgroup. ────
      // The picker is a Radix Dialog with focus-trap + aria-hidden on the rest
      // of the page, so we must not assert against headings outside the dialog
      // while the picker is open. Assert the picker title (inside the dialog)
      // first, click the cardgroup, and only then verify the underlying form.
      const dialog = page.getByRole("dialog", { name: "Select cardgroup" });
      await expect(dialog).toBeVisible();
      await dialog.getByRole("button", { name: cardgroupName }).click();

      // ── 5. URL gains ?cardgroup=<id> and the chip is pre-selected. ───────
      await page.waitForURL(`**/cards/new?cardgroup=${newCardgroupId}`, {
        timeout: 10_000,
      });

      // With the dialog closed, the underlying page is back in the a11y tree.
      await expect(page.getByRole("heading", { name: "New card" })).toBeVisible();

      // The CardgroupChip's aria-label is "Change cardgroup (currently \"<name>\")"
      // when a cardgroup is selected; the "Select cardgroup" placeholder label
      // means no cardgroup is selected. Asserting the populated form proves the
      // pre-select happened.
      await expect(
        page.getByRole("button", {
          name: `Change cardgroup (currently "${cardgroupName}")`,
        }),
      ).toBeVisible();

      // The CardForm should also now be rendered (currentId != null branch),
      // so the Front input is interactable — a stronger signal than chip text
      // alone that the resolved cardgroupId reached CardsNewClient.
      await expect(page.getByLabel("Front")).toBeVisible();
    });
  });
