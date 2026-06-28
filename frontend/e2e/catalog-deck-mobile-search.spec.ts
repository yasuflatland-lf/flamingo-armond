import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedMaster, seedUser } from "./_auth";

// The header search takeover is mobile-only (md:hidden). The default e2e
// viewport is Desktop Chrome (1280px), so this scenario overrides to a phone
// viewport (390x844), mirroring cardgroups-mobile-search.spec.ts.
//
// Reachability guard for the public catalog deck-detail route (/catalog/[id]):
// the screen renders its card list directly and wires the header-takeover
// filter, so resolveHeaderSearchAction must light the mobile magnifier here. On
// mobile the desktop CardSearchInput is hidden, so the header trigger is the
// only search affordance — if it is unwired, mobile users cannot search at all.
//
// Select by data-testid / role — e2e renders in the ja-JP locale, so never match
// on translated copy. The empty-search assertion keys off the
// `catalog-deck-empty-search` testid rather than card text, which keeps it both
// locale-independent and immune to the Suspense-streaming hidden-buffer
// double-match described in cardgroups-mobile-search.spec.ts.
test.describe("catalog deck-detail mobile search takeover", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  const runId = randomUUID().slice(0, 8);
  const user = {
    email: `catalog-mobile-search-${runId}@example.test`,
    password: "e2e-password",
  };
  let masterId: string;

  test.beforeAll(async () => {
    await seedUser({
      email: user.email,
      password: user.password,
      role: "general",
      displayName: "E2E Catalog Search",
    });
    const master = await seedMaster({
      name: `Catalog Search ${runId}`,
      status: "PUBLISHED",
      cards: [
        { front: `Alpha ${runId}`, back: "first card" },
        { front: `Beta ${runId}`, back: "second card" },
      ],
    });
    masterId = master.id;
  });

  test("opens the takeover from the header, filters to empty, and closes", async ({
    context,
    page,
  }) => {
    await loginAs(context, { email: user.email, password: user.password });
    await page.goto(`/catalog/${masterId}`);

    // The deck rendered with its seeded cards.
    await expect(page.getByTestId("catalog-deck-card-list")).toBeVisible();

    // The fix under test: the mobile magnifier appears on /catalog/[id].
    await page.getByTestId("header-search-trigger").click();
    const input = page.getByTestId("search-takeover-input");
    await expect(input).toBeVisible();

    // A non-matching query round-trips to the backend and yields the empty-search
    // state — proves search is wired end-to-end on this screen.
    await input.fill(`zzz-no-match-${runId}`);
    await expect(page.getByTestId("catalog-deck-empty-search")).toBeVisible();

    // Close: the bar disappears; the active dot remains because the query is set.
    await page.getByTestId("search-takeover-close").click();
    await expect(page.getByTestId("search-takeover")).toBeHidden();
    await expect(page.getByTestId("header-search-active-dot")).toBeVisible();
  });
});
