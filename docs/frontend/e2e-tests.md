# E2E tests

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Playwright specs live in `frontend/e2e/` and use `frontend/playwright.config.ts` (which builds the app and serves it via `next start`). Run `pnpm --filter frontend test:e2e` after starting Supabase and the Go backend. The service-role key must stay in `E2E_SUPABASE_SERVICE_ROLE_KEY` and never be exposed as `NEXT_PUBLIC_*`. See `docs/e2e.md` for the auth helper, local run setup, and CI flow.

## Disambiguate locators when breakpoint-distinct affordances share visible text

A `hidden md:inline-flex` desktop-only button and a breakpoint-agnostic empty-state CTA can both carry the same visible text (e.g. "New cardgroup"). At Playwright's Desktop Chrome viewport (1280 px, above the `md` breakpoint) both elements render in the accessibility tree, so `page.getByRole("button", { name: /New cardgroup/ })` matches two elements. `toBeVisible()` then throws a strict-mode error, and `.toHaveCount(1)` fails.

**Rule:** target each affordance by its `data-testid` (e.g. `cardgroups-header-new-btn` for the desktop header button, `cardgroups-empty-cta` for the empty-state CTA). Reserve a text/name regex only for a deliberate co-existence assertion — for example `.toHaveCount(2)` that documents "both affordances exist at this viewport".

**Casing subtlety worth noting:** a regex like `/New cardgroup/` (capital N) does not match an `aria-label` of "Add new cardgroup" (lowercase n), so the nav-header icon button is excluded from the count. Relying on that casing accident is fragile; `data-testid` targeting removes the ambiguity entirely.

Real instance: `frontend/e2e/cardgroups-flow.spec.ts`.

## Multi-step form: assert the closed/done signal on a step-specific element, not the prior step's absence

When advancing from step 1 to step 2 unmounts step 1's DOM nodes, any step-1 element immediately disappears — regardless of whether the whole form later closes. Asserting "step-1 textarea is hidden ⇒ the sheet closed" is invalid: the textarea is gone the moment step 2 renders, so it cannot distinguish "advanced to step 2" from "sheet closed". Target an element owned by the **final step** for the close/done signal.

Worked example: `frontend/e2e/cardgroup-import.spec.ts` asserts `await expect(page.getByRole("button", { name: /Import 2 cards/ })).toBeHidden()` as the full-success close signal, because the step-2 import button disappears only when the sheet unmounts on close — the step-1 textarea had already disappeared when step 2 was reached and is therefore not a valid closed signal (see the inline comment at lines 79–82).

