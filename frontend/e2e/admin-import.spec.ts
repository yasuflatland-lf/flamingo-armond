import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

const runId = Date.now().toString(36);
const admin = {
  email: `admin-import-${runId}@example.test`,
  password: "e2e-password",
};

let cardgroup: Awaited<ReturnType<typeof seedCardgroup>>;

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
  const frontA = `e2ealpha${runId}`;
  const frontB = `e2ebeta${runId}`;
  const defA = String.fromCodePoint(0x308a, 0x3093, 0x3054);
  const defB = String.fromCodePoint(0x72ac);
  const payload = `${frontA} ${defA}\n${frontB} ${defB}`;

  await page.goto("/admin/dictionary");
  await expect(page.getByRole("heading", { name: "Dictionary Import" })).toBeVisible();

  await page.getByLabel("Target cardgroup").selectOption({ label: cardgroup.name });
  await page.getByLabel("Dictionary payload").fill(payload);
  await page.getByRole("button", { name: "Validate" }).click();

  await expect(page.getByRole("status").filter({ hasText: /^Valid/ })).toBeVisible();
  await expect(page.getByRole("cell", { name: frontA })).toBeVisible();
  await expect(page.getByRole("cell", { name: frontB })).toBeVisible();

  await page.getByRole("button", { name: "Import" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Import complete" })).toContainText(
    "2 inserted",
  );

  await page.goto(`/cardgroups/${cardgroup.id}/cards`);
  await expect(page.getByRole("heading", { name: `Cards in ${cardgroup.name}` })).toBeVisible();
  await expect(page.getByText(frontA)).toBeVisible();
  await expect(page.getByText(frontB)).toBeVisible();
});
