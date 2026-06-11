import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedUser } from "./_auth";

/**
 * Language round-trip: sign in → /profile → assert ja initial state →
 * switch to English → assert lang=en + English nav → switch back to Japanese →
 * assert lang=ja + Japanese nav.
 *
 * The LanguageSwitcher is a Radix UI Select (role=combobox) on /profile.
 * The suite's Playwright locale is ja-JP, so the page starts in Japanese.
 */

const runId = randomUUID().slice(0, 8);
const user = {
  email: `i18n-switch-${runId}@example.test`,
  password: "e2e-password",
};

test.describe
  .serial("language round-trip", () => {
    test.beforeAll(async () => {
      await seedUser({
        email: user.email,
        password: user.password,
        role: "general",
        displayName: `I18n Switch ${runId}`,
      });
    });

    test("Japanese initial state, switch to English and back", async ({ context, page }) => {
      await loginAs(context, user);
      await page.goto("/profile");
      await page.waitForURL("**/profile", { timeout: 15_000 });

      // ── Assert initial Japanese state ───────────────────────────────────────
      // Playwright locale ja-JP → Accept-Language: ja → resolveLocale → "ja"
      await expect(page.locator("html")).toHaveAttribute("lang", "ja", { timeout: 10_000 });
      await expect(page.getByRole("link", { name: "カードグループ" })).toBeVisible();

      // ── Switch to English ───────────────────────────────────────────────────
      // The LanguageSwitcher <SelectTrigger> renders as role=combobox.
      // There is exactly one combobox on /profile (the language picker).
      const langTrigger = page.getByRole("combobox");
      await expect(langTrigger).toBeVisible();
      await langTrigger.click();

      // SelectContent renders options as role=option in a portal.
      const englishOption = page.getByRole("option", { name: "English" });
      await expect(englishOption).toBeVisible({ timeout: 5_000 });
      await englishOption.click();

      // setUserLocale server action sets the NEXT_LOCALE cookie; Next.js re-renders.
      await expect(page.locator("html")).toHaveAttribute("lang", "en", { timeout: 15_000 });
      await expect(page.getByRole("link", { name: "Cardgroups" })).toBeVisible();

      // ── Switch back to Japanese ─────────────────────────────────────────────
      await langTrigger.click();
      const jaOption = page.getByRole("option", { name: "日本語" });
      await expect(jaOption).toBeVisible({ timeout: 5_000 });
      await jaOption.click();

      await expect(page.locator("html")).toHaveAttribute("lang", "ja", { timeout: 15_000 });
      await expect(page.getByRole("link", { name: "カードグループ" })).toBeVisible();
    });
  });
