import { expect, test } from "@playwright/test";
import { loginAs, seedMaster, seedUser } from "./_auth";

test.describe("admin master edit navigation", () => {
  test("clicking a master row navigates to its edit page", async ({ context, page }) => {
    const runId = `${Date.now()}-${test.info().workerIndex}`;
    const admin = await seedUser({
      email: `admin-edit-${runId}@example.com`,
      password: "Password123!",
      role: "admin",
      displayName: "Admin Edit",
    });
    const deck = await seedMaster({
      name: `E2E Deck ${runId}`,
      cards: [{ front: "hola", back: "hello" }],
    });

    await loginAs(context, { email: admin.email, password: admin.password });
    await page.goto("/admin/masters");

    // Locale-independent selector (e2e runs in ja-JP): the row testid carries the id.
    await page.getByTestId(`master-catalog-row-${deck.id}`).click();

    await page.waitForURL(new RegExp(`/admin/masters/${deck.id}/edit$`), { timeout: 15_000 });
    // Deck-settings opens the metadata form (select by testid, not translated label).
    await page.getByTestId("master-edit-deck-settings").click();
    await expect(page.getByTestId("master-field-name")).toBeVisible();
  });
});
