import { expect, test } from "@playwright/test";
import { loginAs, seedCardgroup, seedCards, seedUser } from "./_auth";

const runId = Date.now().toString(36);
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
    Array.from({ length: 6 }, (_, index) => ({
      cardgroupId: cardgroup.id,
      front: `learn-${runId}-${index + 1}`,
      back: `answer-${index + 1}`,
    })),
  );
});

test.beforeEach(async ({ context }) => {
  await loginAs(context, learner);
});

test("swipes easy cards and shows adaptive mode feedback", async ({ page }) => {
  await page.goto(`/learn/${cardgroup.id}`);

  await expect(page.getByText(cardgroup.name)).toBeVisible();
  const activeCard = page.locator('[data-testid="swipe-card"][tabindex="0"]');
  await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: learn-${runId}-`));
  await expect(page.getByText("0 completed / 6 remaining")).toBeVisible();

  for (let completed = 1; completed <= 3; completed += 1) {
    await page.getByRole("button", { name: "Easy" }).click();
    await expect(
      page.getByText(`${completed} completed / ${6 - completed} remaining`),
    ).toBeVisible();
    await expect(page.getByText("Ready")).toBeVisible();
  }

  await expect(page.getByText("Mode: Default")).toBeVisible();
  await expect(page.getByText("100% success")).toBeVisible();
  await expect(activeCard).toHaveAccessibleName(new RegExp(`^Flashcard: learn-${runId}-`));
});
