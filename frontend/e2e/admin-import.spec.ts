import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

const runId = randomUUID().slice(0, 8);
const admin = {
  email: `admin-import-${runId}@example.test`,
  password: "e2e-password",
};

let cardgroup: Awaited<ReturnType<typeof seedCardgroup>>;

const frontA = `e2ealpha${runId}`;
const frontB = `e2ebeta${runId}`;
// Unicode codepoints for definitions (apple / dog in Japanese)
const defA = String.fromCodePoint(0x308a, 0x3093, 0x3054);
const defB = String.fromCodePoint(0x72ac);
const payload = `${frontA}\t${defA}\n${frontB}\t${defB}`;

test.describe
  .serial("admin dictionary import", () => {
    test.beforeAll(async () => {
      const user = await seedUser({
        email: admin.email,
        password: admin.password,
        role: "admin",
        displayName: "E2E Admin Import",
      });
      cardgroup = await seedCardgroup({
        ownerId: user.id,
        name: `E2E import ${runId}`,
      });
    });

    test.beforeEach(async ({ context }) => {
      await loginAs(context, admin);
    });

    test("validates and imports a dictionary payload", async ({ page }) => {
      const response = await page.goto("/admin/dictionary");
      expect(response?.ok(), `goto returned status ${response?.status()}`).toBe(true);
      await expect(page.getByRole("heading", { name: "Dictionary Import" })).toBeVisible();

      await page.getByLabel("Target cardgroup").selectOption({ label: cardgroup.name });
      await page.getByLabel("Dictionary payload").fill(payload);
      await page.getByRole("button", { name: "Validate" }).click();

      await expect(page.getByRole("status").filter({ hasText: /^Valid.*2/ })).toBeVisible();
      await expect(page.getByRole("cell", { name: frontA })).toBeVisible();
      await expect(page.getByRole("cell", { name: frontB })).toBeVisible();

      await page.getByRole("button", { name: "Import" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Import complete" })).toContainText(
        "2 inserted",
      );

      const cardsResponse = await page.goto(`/cardgroups/${cardgroup.id}/cards`);
      expect(cardsResponse?.ok(), `goto returned status ${cardsResponse?.status()}`).toBe(true);
      await expect(page.getByRole("heading", { name: `Cards in ${cardgroup.name}` })).toBeVisible();
      await expect(page.getByText(frontA)).toBeVisible();
      await expect(page.getByText(frontB)).toBeVisible();
    });

    test("re-importing the same payload reports updates, not inserts", async ({ page }) => {
      const response = await page.goto("/admin/dictionary");
      expect(response?.ok(), `goto returned status ${response?.status()}`).toBe(true);
      await expect(page.getByRole("heading", { name: "Dictionary Import" })).toBeVisible();

      await page.getByLabel("Target cardgroup").selectOption({ label: cardgroup.name });
      await page.getByLabel("Dictionary payload").fill(payload);
      await page.getByRole("button", { name: "Validate" }).click();
      await expect(page.getByRole("status").filter({ hasText: /^Valid.*2/ })).toBeVisible();

      await page.getByRole("button", { name: "Import" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Import complete" })).toBeVisible();

      await expect(page.getByRole("status").filter({ hasText: "Import complete" })).toContainText(
        /0 inserted.*2 updated/,
      );
    });
  });
