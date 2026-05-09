import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedCards, seedUser } from "./_auth";

// Scenario: a returning user who has already opened a cardgroup on /learn lands
// directly on /learn/<lastViewedId> on the next login. After switching to a
// different cardgroup the next session must follow the most recently viewed
// one, proving setLastViewedCardgroup persists across logout/login cycles.

const runId = randomUUID().slice(0, 8);
const learner = {
  email: `returning-learn-${runId}@example.test`,
  password: "e2e-password",
};
const SWIPE_CARD = '[data-testid="swipe-card"]';

let groupA: Awaited<ReturnType<typeof seedCardgroup>>;
let groupB: Awaited<ReturnType<typeof seedCardgroup>>;

test.describe
  .serial("returning user learn redirect", () => {
    test.beforeAll(async () => {
      const user = await seedUser({
        email: learner.email,
        password: learner.password,
        role: "general",
        displayName: "E2E Returning Learner",
      });
      groupA = await seedCardgroup({
        ownerId: user.id,
        name: `E2E learn A ${runId}`,
      });
      groupB = await seedCardgroup({
        ownerId: user.id,
        name: `E2E learn B ${runId}`,
      });
      await seedCards([
        { cardgroupId: groupA.id, front: `a-${runId}-1`, back: "answer-a-1" },
        { cardgroupId: groupA.id, front: `a-${runId}-2`, back: "answer-a-2" },
        { cardgroupId: groupB.id, front: `b-${runId}-1`, back: "answer-b-1" },
        { cardgroupId: groupB.id, front: `b-${runId}-2`, back: "answer-b-2" },
      ]);
    });

    test("home redirects to the most recently visited cardgroup across sessions", async ({
      context,
      page,
    }) => {
      // Session 1: warm last_viewed by visiting /learn/A then /learn/B.
      await loginAs(context, learner);

      // User has cardgroups but no last_viewed yet → home redirects to /cardgroups.
      await page.goto("/");
      await page.waitForURL("**/cardgroups", { timeout: 10_000 });

      // LearnClient's mount effect fires setLastViewedCardgroup, persisting last_viewed server-side.
      // The post-refactor learn page no longer renders the cardgroup name, so we
      // confirm the correct cardgroup landed by matching the active card's
      // aria-label against the per-cardgroup `runId` front prefix from seedCards.
      const activeCard = page.locator(`${SWIPE_CARD}[tabindex="0"]`);
      const responseA = await page.goto(`/learn/${groupA.id}`);
      expect(responseA?.ok(), `goto /learn/A returned ${responseA?.status()}`).toBe(true);
      await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: a-${runId}-`));

      // Switch to /learn/B; this re-fires the mutation with cardgroupId=B.
      // Arm waitForResponse BEFORE navigation: LearnClient's mount effect can
      // resolve the mutation before a post-goto listener attaches, leaving the
      // listener waiting for a "next" SetLastViewedCardgroup that never fires.
      // waitForResponse only matches responses arriving after it is awaited, so
      // the promise must be created before page.goto to bracket the request.
      const persistB = page.waitForResponse(
        (res) =>
          res.url().includes("/api/graphql") &&
          res.request().postDataJSON()?.operationName === "SetLastViewedCardgroup",
      );
      const responseB = await page.goto(`/learn/${groupB.id}`);
      expect(responseB?.ok(), `goto /learn/B returned ${responseB?.status()}`).toBe(true);
      await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: b-${runId}-`));

      // Deterministically wait for the persist mutation to land before logout.
      // Keying on the operation name avoids the racy networkidle (500ms idle),
      // which can return early under Apollo's async cache-write timeline and
      // hide persistence regressions.
      await persistB;

      // Logout: clear cookies so the next login starts cold.
      await context.clearCookies();
      await page.goto("/");
      await page.waitForURL("**/login", { timeout: 10_000 });

      // Session 2: re-login and verify home redirects to /learn/B.
      await loginAs(context, learner);
      await page.goto("/");
      await page.waitForURL(`**/learn/${groupB.id}`, { timeout: 10_000 });

      // Confirm cardgroup B actually painted (waitForURL alone does not assert paint).
      // Match the active card's aria-label rather than the cardgroup name, which
      // the post-refactor learn page no longer renders.
      await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: b-${runId}-`));
    });
  });
