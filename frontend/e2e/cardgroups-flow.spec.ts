import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

// Covers the cardgroup-creation navigation refactor:
//   - Primary action on /cardgroups (with at least one cardgroup) and empty state
//   - Picker-driven create flow from /cards/new
//   - Open-redirect rejection for external returnTo values

const runId = randomUUID().slice(0, 8);
const UUID_RE = /^[0-9a-f-]{36}$/;

const withCardgroup = {
  email: `cg-flow-${runId}@example.test`,
  password: "e2e-password",
};

let seededCardgroupName: string;

const emptyUser = {
  email: `cg-empty-${runId}@example.test`,
  password: "e2e-password",
};

test.describe
  .serial("cardgroups flow", () => {
    test.beforeAll(async () => {
      // Seed the "with cardgroup" user and one cardgroup.
      const user = await seedUser({
        email: withCardgroup.email,
        password: withCardgroup.password,
        role: "general",
        displayName: "E2E CG Flow",
      });
      const cg = await seedCardgroup({
        ownerId: user.id,
        name: `E2E cardgroup ${runId}`,
      });
      seededCardgroupName = cg.name;

      // Seed the "empty" user with no cardgroups.
      await seedUser({
        email: emptyUser.email,
        password: emptyUser.password,
        role: "general",
        displayName: "E2E CG Empty",
      });
    });

    // ── Scenario 1 ──────────────────────────────────────────────────────────────
    // Primary action on /cardgroups: with at least one cardgroup the "New
    // cardgroup" button is visible in the page-shell header (primaryActions).
    // Clicking it opens the create-cardgroup drawer (FormSheet) in place — no
    // navigation. The full-page /cardgroups/new route is reserved for the
    // onboarding (welcome) and picker returnTo flows (Scenario 3).
    test("primary action on /cardgroups opens the create-cardgroup drawer in place", async ({
      context,
      page,
    }) => {
      await loginAs(context, withCardgroup);

      const response = await page.goto("/cardgroups");
      expect(response?.ok(), `goto /cardgroups returned ${response?.status()}`).toBe(true);

      // The seeded cardgroup must be visible, confirming the non-empty branch renders.
      await expect(page.getByText(seededCardgroupName)).toBeVisible();

      // The primary "New cardgroup" action is a button (not a link) — it opens
      // the drawer in place rather than navigating. The desktop button carries
      // `hidden md:inline-flex`; Playwright's Desktop Chrome viewport (1280px) is
      // above the md breakpoint so the button is visible.
      // Use data-testid to avoid locale-dependent button text (Playwright runs ja-JP).
      const newCardgroupButton = page.getByTestId("cardgroups-header-new-btn");
      await expect(newCardgroupButton).toBeVisible();

      // Click it and verify the FormSheet opens in place with the form interactive.
      // Check the Name input (CardgroupForm label — not yet localized) rather than
      // the translated sheet heading.
      await newCardgroupButton.click();
      await expect(page.locator('input[name="name"]')).toBeVisible();
      await expect(page.getByRole("button", { name: "Create" })).toBeVisible();

      // The URL must NOT have navigated — the in-place drawer is the entire point.
      expect(new URL(page.url()).pathname).toBe("/cardgroups");
    });

    // ── Scenario 2 ──────────────────────────────────────────────────────────────
    // Empty-state on /cardgroups: with zero cardgroups the "No cardgroups yet"
    // copy is shown. Two "New cardgroup" affordances coexist in this branch:
    //   - the page-shell header primary action (`cardgroups-header-new-btn`,
    //     `hidden md:inline-flex`, visible at md+), and
    //   - the empty-state CTA (`cardgroups-empty-cta`), which renders only when
    //     the list is empty and is breakpoint-agnostic (visible at all widths).
    // Both carry the text "New cardgroup", so a `/New cardgroup/` name regex
    // matches both. Target each by data-testid to keep the assertions unambiguous.
    test("empty state on /cardgroups shows the empty-state copy and primary action", async ({
      context,
      page,
    }) => {
      await loginAs(context, emptyUser);

      const response = await page.goto("/cardgroups");
      expect(response?.ok(), `goto /cardgroups returned ${response?.status()}`).toBe(true);

      // The empty-state container confirms we are in the zero-cardgroup branch.
      // Use data-testid to avoid locale-dependent text (Playwright runs ja-JP).
      await expect(page.getByTestId("cardgroups-empty")).toBeVisible();

      // The page-shell header primary action carries `hidden md:inline-flex`;
      // Playwright's Desktop Chrome viewport (1280px) is above the md breakpoint
      // so the desktop header button is visible. (The nav-header "+" carries
      // aria-label "Add new cardgroup", distinguished from the `/New cardgroup/`
      // regex by the lowercase 'n' in "new" — it is not asserted here.)
      await expect(page.getByTestId("cardgroups-header-new-btn")).toBeVisible();

      // The empty-state CTA also renders in the zero-cardgroup branch and is
      // breakpoint-agnostic, so it is likewise visible at the desktop viewport.
      await expect(page.getByTestId("cardgroups-empty-cta")).toBeVisible();

      // Verify exactly two "New cardgroup" affordances: header and empty-state CTA.
      await expect(page.getByTestId("cardgroups-header-new-btn")).toHaveCount(1);
      await expect(page.getByTestId("cardgroups-empty-cta")).toHaveCount(1);
    });

    // ── Scenario 3 ──────────────────────────────────────────────────────────────
    // Picker-driven create flow: /cards/new auto-opens the picker (user has one
    // cardgroup but no lastViewed → forcePickerOpen=true Priority 3 in
    // app/cards/new/page.tsx) → "Create new cardgroup…" link →
    // /cardgroups/new?returnTo=%2Fcards%2Fnew → fill name → submit →
    // /cards/new?cardgroup=<newId> with the form interactive and chip pre-selected.
    test("picker 'Create new cardgroup…' link returns to /cards/new with new cardgroup pre-selected", async ({
      context,
      page,
    }) => {
      await loginAs(context, withCardgroup);

      const response = await page.goto("/cards/new");
      expect(response?.ok(), `goto /cards/new returned ${response?.status()}`).toBe(true);

      // The picker auto-opens (Radix Dialog with focus-trap + aria-hidden on
      // the rest of the page), so the "New card" heading and the underlying
      // chip are not in the accessibility tree until the picker closes.
      // Assert the dialog first; the underlying page is verified below after
      // navigation back to /cards/new with ?cardgroup=<newId>.
      // Use data-testid to avoid locale-dependent dialog name (Playwright runs ja-JP).
      const dialog = page.getByTestId("cardgroup-picker-dialog");
      await expect(dialog).toBeVisible();

      // The "Create new cardgroup…" inline link must be present inside the sheet.
      // Use href to avoid locale-dependent link text.
      const createLink = dialog.locator('a[href*="/cardgroups/new"]');
      await expect(createLink).toBeVisible();

      // Clicking the link navigates to /cardgroups/new?returnTo=%2Fcards%2Fnew.
      await createLink.click();
      await page.waitForURL("**/cardgroups/new?returnTo=%2Fcards%2Fnew", { timeout: 10_000 });
      // URL confirms navigation; verify the page heading via data-testid.
      await expect(page.getByTestId("new-cardgroup-page-heading")).toBeVisible();

      // Fill and submit the new cardgroup form.
      const newCgName = `E2E picker-return ${runId}`;
      // Use name attribute to avoid locale-dependent label text (Playwright runs ja-JP).
      await page.locator('input[name="name"]').fill(newCgName);
      await page.getByRole("button", { name: "Create" }).click();

      // After successful creation the router pushes /cards/new?cardgroup=<newId>.
      await page.waitForURL(/\/cards\/new\?cardgroup=[0-9a-f-]{36}/, { timeout: 15_000 });

      // Verify the cardgroup id is a valid UUID.
      const url = new URL(page.url());
      const newCgId = url.searchParams.get("cardgroup") ?? "";
      expect(newCgId).toMatch(UUID_RE);

      // The card form heading and the Name field must now be interactive.
      // Use data-testid / name attribute to avoid locale-dependent text (Playwright runs ja-JP).
      await expect(page.getByTestId("cards-new-page-heading")).toBeVisible();
      await expect(page.locator('input[name="front"]')).toBeVisible();

      // The chip must reflect the newly created cardgroup.
      await expect(
        page.getByRole("button", {
          name: `Change cardgroup (currently "${newCgName}")`,
        }),
      ).toBeVisible();
    });

    // ── Scenario 4 ──────────────────────────────────────────────────────────────
    // Open-redirect rejection: a protocol-relative ("//evil.com") or absolute
    // ("https://evil.com") returnTo is silently stripped by sanitizeReturnTo.
    // After creation the router falls back to /cardgroups/<newId> — NOT to an
    // external origin and NOT to /cards/new.
    test("open-redirect: external returnTo is rejected and falls back to /cardgroups/<id>", async ({
      context,
      page,
    }) => {
      await loginAs(context, withCardgroup);

      // ── 4a: protocol-relative URL ──────────────────────────────────────────────
      const response4a = await page.goto("/cardgroups/new?returnTo=//evil.com");
      expect(response4a?.ok(), `goto returned ${response4a?.status()}`).toBe(true);
      await expect(page.getByTestId("new-cardgroup-page-heading")).toBeVisible();

      const name4a = `E2E redirect-a ${runId}`;
      await page.locator('input[name="name"]').fill(name4a);
      await page.getByRole("button", { name: "Create" }).click();

      // Must land on /cardgroups/<uuid>, never on evil.com or /cards/new.
      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}$/, { timeout: 15_000 });
      const finalUrl4a = page.url();
      expect(finalUrl4a).not.toContain("evil.com");
      expect(finalUrl4a).not.toMatch(/\/cards\/new/);
      const createdId4a = finalUrl4a.split("/").pop() ?? "";
      expect(createdId4a).toMatch(UUID_RE);

      // ── 4b: absolute https URL ─────────────────────────────────────────────────
      const response4b = await page.goto("/cardgroups/new?returnTo=https://evil.com");
      expect(response4b?.ok(), `goto returned ${response4b?.status()}`).toBe(true);
      await expect(page.getByTestId("new-cardgroup-page-heading")).toBeVisible();

      const name4b = `E2E redirect-b ${runId}`;
      await page.locator('input[name="name"]').fill(name4b);
      await page.getByRole("button", { name: "Create" }).click();

      await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}$/, { timeout: 15_000 });
      const finalUrl4b = page.url();
      expect(finalUrl4b).not.toContain("evil.com");
      expect(finalUrl4b).not.toMatch(/\/cards\/new/);
      const createdId4b = finalUrl4b.split("/").pop() ?? "";
      expect(createdId4b).toMatch(UUID_RE);
    });
  });
