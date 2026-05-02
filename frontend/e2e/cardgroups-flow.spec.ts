import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedUser } from "./_auth";

// Covers the cardgroup-creation navigation refactor:
//   - Footer link on /cardgroups (with at least one cardgroup)
//   - Empty-state CTA on /cardgroups (zero cardgroups)
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

test.describe.serial("cardgroups flow", () => {
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
  // Footer-link path on /cardgroups: with at least one cardgroup the footer
  // "New cardgroup" link is visible at the bottom of the list. Clicking it
  // lands on /cardgroups/new with no returnTo, and the form is interactive.
  test("footer link on /cardgroups leads to /cardgroups/new without returnTo", async ({
    context,
    page,
  }) => {
    await loginAs(context, withCardgroup);

    const response = await page.goto("/cardgroups");
    expect(response?.ok(), `goto /cardgroups returned ${response?.status()}`).toBe(true);

    // The seeded cardgroup must be visible, confirming the non-empty branch renders.
    await expect(page.getByText(seededCardgroupName)).toBeVisible();

    // The large dashed-border CTA must NOT be present in the non-empty branch.
    await expect(
      page.getByRole("link", { name: "New cardgroup" }).filter({ hasText: "New cardgroup" }).first(),
    ).toBeVisible();

    // Locate the footer link specifically — it lives inside the border-t container
    // after the list, not the empty-state CTA. Both branches use the same link
    // text so we scope to the wrapper that renders only in the non-empty branch.
    const footerScope = page.locator(".mt-6.border-t.pt-4");
    const footerLink = footerScope.getByRole("link", { name: /New cardgroup/ });
    await expect(footerLink).toBeVisible();
    await expect(footerLink).toHaveAttribute("href", "/cardgroups/new");

    // Click it and verify we land on /cardgroups/new with the form interactive.
    await footerLink.click();
    await page.waitForURL("**/cardgroups/new", { timeout: 10_000 });
    await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();
    await expect(page.getByLabel("Name")).toBeVisible();
    await expect(page.getByRole("button", { name: "Create" })).toBeVisible();
  });

  // ── Scenario 2 ──────────────────────────────────────────────────────────────
  // Empty-state on /cardgroups: with zero cardgroups the dashed-border large CTA
  // is the only "New cardgroup" affordance; the footer link is not present.
  test("empty state on /cardgroups shows only the dashed CTA, not the footer link", async ({
    context,
    page,
  }) => {
    await loginAs(context, emptyUser);

    const response = await page.goto("/cardgroups");
    expect(response?.ok(), `goto /cardgroups returned ${response?.status()}`).toBe(true);

    // The empty-state description text confirms we are in the zero-cardgroup branch.
    await expect(
      page.getByText("You haven't created any cardgroups yet."),
    ).toBeVisible();

    // The dashed-border CTA link must be visible.
    const ctaLink = page.getByRole("link", { name: "New cardgroup" });
    await expect(ctaLink).toBeVisible();

    // There must be exactly ONE "New cardgroup" link — the footer link is absent
    // in the empty-state branch.
    await expect(ctaLink).toHaveCount(1);
  });

  // ── Scenario 3 ──────────────────────────────────────────────────────────────
  // Picker-driven create flow: /cards/new → open picker → "Create new cardgroup…"
  // → /cardgroups/new?returnTo=%2Fcards%2Fnew → fill name → submit →
  // /cards/new?cardgroup=<newId> with the form interactive and chip pre-selected.
  test("picker 'Create new cardgroup…' link returns to /cards/new with new cardgroup pre-selected", async ({
    context,
    page,
  }) => {
    await loginAs(context, withCardgroup);

    const response = await page.goto("/cards/new");
    expect(response?.ok(), `goto /cards/new returned ${response?.status()}`).toBe(true);
    await expect(page.getByRole("heading", { name: "New card" })).toBeVisible();

    // Open the cardgroup picker by clicking the chip.
    const chip = page.getByRole("button", { name: /Select cardgroup|Change cardgroup/ });
    await expect(chip).toBeVisible();
    await chip.click();

    // The picker renders as a bottom Sheet (role="dialog", title "Select cardgroup").
    const dialog = page.getByRole("dialog", { name: "Select cardgroup" });
    await expect(dialog).toBeVisible();

    // The "Create new cardgroup…" inline link must be present inside the sheet.
    const createLink = dialog.getByRole("link", { name: /Create new cardgroup/ });
    await expect(createLink).toBeVisible();

    // Clicking the link navigates to /cardgroups/new?returnTo=%2Fcards%2Fnew.
    await createLink.click();
    await page.waitForURL("**/cardgroups/new?returnTo=%2Fcards%2Fnew", { timeout: 10_000 });
    await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();

    // Fill and submit the new cardgroup form.
    const newCgName = `E2E picker-return ${runId}`;
    await page.getByLabel("Name").fill(newCgName);
    await page.getByRole("button", { name: "Create" }).click();

    // After successful creation the router pushes /cards/new?cardgroup=<newId>.
    await page.waitForURL(/\/cards\/new\?cardgroup=[0-9a-f-]{36}/, { timeout: 15_000 });

    // Verify the cardgroup id is a valid UUID.
    const url = new URL(page.url());
    const newCgId = url.searchParams.get("cardgroup") ?? "";
    expect(newCgId).toMatch(UUID_RE);

    // The card form heading and the Name field must now be interactive.
    await expect(page.getByRole("heading", { name: "New card" })).toBeVisible();
    await expect(page.getByLabel("Front")).toBeVisible();

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
    await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();

    const name4a = `E2E redirect-a ${runId}`;
    await page.getByLabel("Name").fill(name4a);
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
    await expect(page.getByRole("heading", { name: "New cardgroup" })).toBeVisible();

    const name4b = `E2E redirect-b ${runId}`;
    await page.getByLabel("Name").fill(name4b);
    await page.getByRole("button", { name: "Create" }).click();

    await page.waitForURL(/\/cardgroups\/[0-9a-f-]{36}$/, { timeout: 15_000 });
    const finalUrl4b = page.url();
    expect(finalUrl4b).not.toContain("evil.com");
    expect(finalUrl4b).not.toMatch(/\/cards\/new/);
    const createdId4b = finalUrl4b.split("/").pop() ?? "";
    expect(createdId4b).toMatch(UUID_RE);
  });
});
