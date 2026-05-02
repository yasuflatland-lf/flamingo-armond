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
      // ── Session 1: warm the lastViewedCardgroup by visiting /learn/A. ─────
      await loginAs(context, learner);

      // The user has cardgroups but no last_viewed yet → home redirects to /cardgroups.
      await page.goto("/");
      await page.waitForURL("**/cardgroups", { timeout: 10_000 });

      // Visit /learn/A directly — LearnClient's mount effect fires
      // setLastViewedCardgroup, which persists last_viewed = A server-side.
      const responseA = await page.goto(`/learn/${groupA.id}`);
      expect(responseA?.ok(), `goto /learn/A returned ${responseA?.status()}`).toBe(true);
      await expect(page.getByText(groupA.name)).toBeVisible();
      // Wait for at least one swipe card so the LearnClient mount effect has
      // had a tick to issue the mutation. The mutation itself is non-blocking
      // (warn-on-failure), so we additionally rely on the home-redirect on the
      // next session to assert the server-side state was actually persisted.
      await expect(page.locator('[data-testid="swipe-card"]').first()).toBeVisible();

      // ── Switch to /learn/B; this re-fires the mutation with cardgroupId=B. ─
      const responseB = await page.goto(`/learn/${groupB.id}`);
      expect(responseB?.ok(), `goto /learn/B returned ${responseB?.status()}`).toBe(true);
      await expect(page.getByText(groupB.name)).toBeVisible();
      await expect(page.locator('[data-testid="swipe-card"]').first()).toBeVisible();

      // Give the mount-effect mutation a brief moment to round-trip before we
      // tear down the session. The mutation has no UI signal, so we wait on a
      // background network idle marker instead of a DOM event.
      await page.waitForLoadState("networkidle");

      // ── Logout: clear cookies so the next login starts from a cold state. ──
      await context.clearCookies();

      // Sanity check: hitting / now bounces to /login.
      await page.goto("/");
      await page.waitForURL("**/login", { timeout: 10_000 });

      // ── Session 2: re-login and verify home redirects to /learn/B. ────────
      await loginAs(context, learner);
      await page.goto("/");
      await page.waitForURL(`**/learn/${groupB.id}`, { timeout: 10_000 });

      // Confirm the page actually rendered the cardgroup B context, not a stale
      // shell of cardgroup A. waitForURL alone does not assert paint.
      await expect(page.getByText(groupB.name)).toBeVisible();
    });
  });
