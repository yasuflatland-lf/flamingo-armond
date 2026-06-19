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
    // Deck settings now lives in the desktop split-button dropdown (e2e runs at a
    // Desktop Chrome viewport), so open the chevron first. Select by testid, not
    // translated label.
    await page.getByTestId("master-edit-more-options").click();
    await page.getByTestId("master-edit-deck-settings").click();
    await expect(page.getByTestId("master-field-name")).toBeVisible();
  });
});

test.describe("admin master card CRUD", () => {
  test("add then edit then delete a card on the master edit page", async ({ context, page }) => {
    const runId = `${Date.now()}-${test.info().workerIndex}`;
    const admin = await seedUser({
      email: `admin-card-crud-${runId}@example.com`,
      password: "Password123!",
      role: "admin",
      displayName: "Admin Card CRUD",
    });
    const deck = await seedMaster({
      name: `CRUD Deck ${runId}`,
      status: "DRAFT",
      cards: [{ front: "seedfront", back: "seedback" }],
    });

    await loginAs(context, { email: admin.email, password: admin.password });
    await page.goto(`/admin/masters/${deck.id}/edit`);
    await expect(page.getByTestId("master-cards-section")).toBeVisible({ timeout: 15_000 });

    // ADD: open the add-card sheet, fill in unique front/back, and submit.
    const cardFront = `e2efront-${runId}`;
    const cardBack = "e2eback";
    await page.getByTestId("master-add-card").click();
    await page.locator("#add-card-front-field").fill(cardFront);
    await page.locator("#add-card-back-field").fill(cardBack);
    await page.locator('[data-testid="form-sheet-body"]').locator('button[type="submit"]').click();
    // Assert the new card row is now visible in the list.
    await expect(page.getByText(cardFront)).toBeVisible({ timeout: 10_000 });

    // Wait for the add-card sheet to finish its close animation and unmount
    // before opening the edit sheet. The add and edit sheets share the same
    // [id$="-back-field"] / form-sheet-body submit shape, so while the add sheet
    // lingers mid-exit the edit-step locators below would match two elements
    // (strict-mode violation). Gating on the add field's removal makes the edit
    // step deterministic.
    await expect(page.locator("#add-card-back-field")).toHaveCount(0);

    // EDIT: click the card's edit-target div to open the edit sheet, change back text, save.
    const editedBack = "e2eback-edited";
    // Locate the card row by front text and click its edit-target.
    const cardRow = page.locator("li").filter({ hasText: cardFront });
    await cardRow.locator('[data-testid^="card-edit-target-"]').click();
    const backInput = page
      .locator('[data-testid="form-sheet-body"]')
      .locator('[id$="-back-field"]');
    await backInput.fill(editedBack);
    await page.locator('[data-testid="form-sheet-body"]').locator('button[type="submit"]').click();
    // Assert the edited back text is visible.
    await expect(page.getByText(editedBack)).toBeVisible({ timeout: 10_000 });

    // DELETE: check the card's select checkbox, trigger bulk delete, confirm.
    const deletedRow = page.locator("li").filter({ hasText: cardFront });
    await deletedRow.locator('[data-testid^="card-select-"]').check();
    await page.getByTestId("cards-bulk-delete-button").click();
    await page.getByTestId("cards-bulk-confirm").click();
    // Assert the card front text is gone from the list.
    await expect(page.getByText(cardFront)).not.toBeVisible({ timeout: 10_000 });
  });
});
