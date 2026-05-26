import { randomUUID } from "node:crypto";
import { expect, type Page, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

const runId = randomUUID().slice(0, 8);
const owner = {
  email: `cardgroup-import-${runId}@example.test`,
  password: "e2e-password",
};

let cardgroup: Awaited<ReturnType<typeof seedCardgroup>>;

const frontA = `e2ealpha${runId}`;
const frontB = `e2ebeta${runId}`;
// Unicode codepoints for the back definitions keep this source ASCII-only.
const defA = String.fromCodePoint(0x308a, 0x3093, 0x3054);
const defB = String.fromCodePoint(0x72ac);
// Re-import overwrites the backs to prove the upsert updates rather than inserts.
const defA2 = String.fromCodePoint(0x732b);
const defB2 = String.fromCodePoint(0x9b5a);
const payload = `${frontA}\t${defA}\n${frontB}\t${defB}`;
const updatedPayload = `${frontA}\t${defA2}\n${frontB}\t${defB2}`;

// The import entry point moved from the removed /admin/dictionary route to the
// owner-authorized cardgroup edit page: open the "Add card" split-button menu
// and choose "Batch import", which opens the import sheet.
async function openBatchImport(page: Page) {
  const response = await page.goto(`/cardgroups/${cardgroup.id}/edit`);
  expect(response?.ok(), `goto returned status ${response?.status()}`).toBe(true);
  await expect(page.getByRole("heading", { level: 1, name: cardgroup.name })).toBeVisible();

  await page.getByRole("button", { name: "More add options" }).click();
  await page.getByRole("menuitem", { name: "Batch import" }).click();
  await expect(page.getByLabel("Cards to import")).toBeVisible();
}

test.describe
  .serial("cardgroup batch import", () => {
    test.beforeAll(async () => {
      // The migrated feature is owner-authorized, not admin-gated; a general user
      // who owns the cardgroup must be able to import into it.
      const user = await seedUser({
        email: owner.email,
        password: owner.password,
        role: "general",
        displayName: "E2E Cardgroup Import",
      });
      cardgroup = await seedCardgroup({
        ownerId: user.id,
        name: `E2E import ${runId}`,
      });
    });

    test.beforeEach(async ({ context }) => {
      await loginAs(context, owner);
    });

    test("validates and imports a card payload", async ({ page }) => {
      await openBatchImport(page);

      await page.getByLabel("Cards to import").fill(payload);
      await page.getByRole("button", { name: "Validate" }).click();

      // Status line now starts with a "✓ " glyph; omit the "^" anchor so the
      // regex matches anywhere in the text content.
      await expect(page.getByRole("status").filter({ hasText: /Valid.*2/ })).toBeVisible();

      // The parse preview is collapsed by default; expand it to verify the cells.
      await page.getByRole("button", { name: /Show preview/ }).click();
      await expect(page.getByRole("cell", { name: frontA })).toBeVisible();
      await expect(page.getByRole("cell", { name: frontB })).toBeVisible();

      // Advance from step 1 "Paste & review" to step 2 "Import".
      await page.getByRole("button", { name: /Continue/ }).click();

      // Step 2: confirm heading and click the import button.
      await page.getByRole("button", { name: /Import 2 cards/ }).click();

      // A full-success import closes the sheet; the whole stepper unmounts so
      // step 2's button disappears. The step-1 textarea is already gone at this
      // point (step 1 unmounts on advancing), so it is not a valid closed signal.
      await expect(page.getByRole("button", { name: /Import 2 cards/ })).toBeHidden();
      await expect(page.getByText(frontA, { exact: true })).toBeVisible();
      await expect(page.getByText(frontB, { exact: true })).toBeVisible();
      await expect(page.getByText(defA, { exact: true })).toBeVisible();
    });

    test("re-importing the same fronts updates the cards without duplicating", async ({ page }) => {
      await openBatchImport(page);

      // Same fronts, different backs: the upsert is keyed on front, so the existing
      // rows are updated rather than duplicated.
      await page.getByLabel("Cards to import").fill(updatedPayload);
      await page.getByRole("button", { name: "Validate" }).click();

      // Status line now starts with a "✓ " glyph; omit the "^" anchor.
      await expect(page.getByRole("status").filter({ hasText: /Valid.*2/ })).toBeVisible();

      // Advance to step 2 and import.
      await page.getByRole("button", { name: /Continue/ }).click();
      await page.getByRole("button", { name: /Import 2 cards/ }).click();
      await expect(page.getByRole("button", { name: /Import 2 cards/ })).toBeHidden();

      // The backs were overwritten (update), and each front still appears exactly
      // once (no duplicate insert).
      await expect(page.getByText(defA2, { exact: true })).toBeVisible();
      await expect(page.getByText(defB2, { exact: true })).toBeVisible();
      await expect(page.getByText(frontA, { exact: true })).toHaveCount(1);
      await expect(page.getByText(frontB, { exact: true })).toHaveCount(1);
    });
  });
