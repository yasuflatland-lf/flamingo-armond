import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

// The header search takeover is mobile-only (md:hidden). The default
// e2e viewport is Desktop Chrome (1280px), so this scenario overrides to a
// phone viewport (390x844), mirroring new-user-onboarding.spec.ts.
// Select by data-testid / role — e2e renders in the ja-JP locale, so never
// match on translated copy.
test.describe("cardgroups mobile search takeover", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  const runId = randomUUID().slice(0, 8);
  const user = {
    email: `cg-mobile-search-${runId}@example.test`,
    password: "e2e-password",
  };

  test.beforeAll(async () => {
    const seeded = await seedUser({
      email: user.email,
      password: user.password,
      role: "general",
      displayName: "E2E Mobile Search",
    });
    await seedCardgroup({ ownerId: seeded.id, name: `Alpha ${runId}` });
    await seedCardgroup({ ownerId: seeded.id, name: `Beta ${runId}` });
  });

  test("opens the takeover from the header, filters, and closes", async ({ context, page }) => {
    await loginAs(context, { email: user.email, password: user.password });
    await page.goto("/cardgroups");

    await expect(page.getByText(`Alpha ${runId}`)).toBeVisible();
    await expect(page.getByText(`Beta ${runId}`)).toBeVisible();

    await page.getByTestId("header-search-trigger").click();
    const input = page.getByTestId("search-takeover-input");
    await expect(input).toBeVisible();

    await input.fill(`Alpha ${runId}`);
    await expect(page.getByText(`Alpha ${runId}`)).toBeVisible();
    await expect(page.getByText(`Beta ${runId}`)).toHaveCount(0);

    // Close: the bar disappears; the active dot remains because the query is set.
    await page.getByTestId("search-takeover-close").click();
    await expect(page.getByTestId("search-takeover")).toBeHidden();
    await expect(page.getByTestId("header-search-active-dot")).toBeVisible();
  });
});
