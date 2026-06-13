import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedCards, seedUser } from "./_auth";

const runId = randomUUID().slice(0, 8);
const learner = {
  email: `learn-flow-${runId}@example.test`,
  password: "e2e-password",
};

let cardgroup: Awaited<ReturnType<typeof seedCardgroup>>;

test.beforeAll(async () => {
  const user = await seedUser({
    email: learner.email,
    password: learner.password,
    role: "general",
    displayName: "E2E Learner",
  });
  cardgroup = await seedCardgroup({
    ownerId: user.id,
    name: `E2E learn ${runId}`,
  });
  await seedCards(
    Array.from({ length: 12 }, (_, index) => ({
      cardgroupId: cardgroup.id,
      front: `learn-${runId}-${index + 1}`,
      back: `answer-${index + 1}`,
    })),
  );
});

test.beforeEach(async ({ context }) => {
  await loginAs(context, learner);
});

test("swipes easy cards and advances through the deck", async ({ page }) => {
  const response = await page.goto(`/learn/${cardgroup.id}`);
  expect(response?.ok(), `goto returned status ${response?.status()}`).toBe(true);

  // The post-refactor learn page renders only the SwipeCardStack — no cardgroup
  // name, no progress bar, no mode badge, no success%, no Saving indicator. We
  // confirm we landed on the right cardgroup via the active card's aria-label,
  // which carries the per-cardgroup `runId` prefix from seedCards.
  const activeCard = page.locator('[data-testid="swipe-card"][tabindex="0"]');
  await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: learn-${runId}-`));

  // The rating buttons moved out of the swipe card into LearnActionBar
  // (sticky bar below the deck), so click at page scope, not inside activeCard.
  // Select by the stable `aria-keyshortcuts` attribute rather than the accessible
  // name: the rating label is now localized and Playwright runs with
  // `locale: "ja-JP"`, so the rendered name is Japanese. The ArrowRight shortcut
  // (the "Easy" rating) is locale-independent.
  const easyButton = page.locator('button[aria-keyshortcuts="ArrowRight"]');

  const seen: string[] = [];
  for (let i = 0; i < 3; i += 1) {
    const before = await activeCard.getAttribute("aria-label");
    if (before) seen.push(before);
    // The default display mode is flip_to_reveal: the back stays hidden and the
    // rating buttons stay disabled until the active card is revealed. Reveal is a
    // card tap / Space on the focused card (never a swipe). Each freshly-advanced
    // card resets to front_only, so reveal again every iteration.
    await activeCard.click();
    await expect(easyButton).toBeEnabled();
    await easyButton.click();
    await expect(activeCard).not.toHaveAttribute("aria-label", before ?? "");
  }

  const after = await activeCard.getAttribute("aria-label");
  expect(seen).not.toContain(after);
  await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: learn-${runId}-`));
});
