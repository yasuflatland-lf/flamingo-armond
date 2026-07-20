# Owner-gated mutations carry no FORBIDDEN-specific banner — the route gate makes the usecase owner-check a backstop

> Part of the [frontend RSC error handling](../../../.claude/rules/frontend-rsc-error-handling.md) rules.

Cardgroup-scoped resources (the `cardgroup(id:)` read, and writes like `updateCardgroup` / `importCards`) deliberately return **`UNAUTHENTICATED`** to a non-owner, never `FORBIDDEN`, so a caller cannot distinguish "exists but not yours" from "not authenticated" and cannot probe existence by ID enumeration. The backend rationale and exact return paths live in [`docs/backend-graphql.md` § "Authorization at the usecase layer"](../../backend-graphql.md#authorization-at-the-usecase-layer) and § "Existence-oracle prevention via collapsed `BAD_USER_INPUT`" — read those for the WHY; this chapter is the presentation-side consequence.

## A client component for an owner-gated mutation must NOT add a FORBIDDEN-specific branch

Two reasons the FORBIDDEN branch is wrong here:

1. **It is dead code.** The backend never emits `FORBIDDEN` for the owner check — it emits `UNAUTHENTICATED`. A client `if (codes.includes("FORBIDDEN"))` banner therefore never fires on the owner-gate path.
2. **Switching the backend to FORBIDDEN to make that branch live would regress the anti-enumeration property.** `FORBIDDEN` ("exists but not yours") is itself an existence oracle. The whole point of returning `UNAUTHENTICATED` is to refuse to confirm the resource exists. So the dead branch must not be "fixed" by changing the backend classification — it must be deleted.

This is the opposite posture from the admin role-CRUD flows in [`UNAUTHENTICATED` vs `FORBIDDEN`: message asymmetry](unauthenticated-vs-forbidden-message-asymmetry.md): there, `FORBIDDEN` carries an actionable owner-specific reason ("cannot delete a protected role") and the asymmetry pays off. Owner-gated cardgroup resources have no such reason to give — confirming the reason would leak existence — so they collapse to `UNAUTHENTICATED` and the client treats every failure uniformly.

## The route gate makes the usecase owner-check a defensive backstop

Because the `cardgroup(id:)` query returns `UNAUTHENTICATED` to non-owners, the owner-gated RSC page redirects on it before any owner-gated UI renders: `frontend/src/app/cardgroups/[id]/edit/page.tsx` catches `isUnauthenticatedGraphQLError(err)` and `redirect("/login")` (and `/cards` redirects to `/edit`). A non-owner therefore never reaches the batch-import form or any owner-gated mutation through the UI. The usecase-layer owner-check (`authorizeCardgroupOrBadInput` in `backend/internal/usecase/ownership.go`, called from `cardImportUsecase.Import`) is a **defensive backstop** reachable only by a hand-crafted GraphQL request that bypasses the RSC redirect — not the primary UX gate.

## Worked example

`frontend/src/components/batch-import/batch-import-wizard.tsx` routes every caught error through `getBackendErrorBanner` (`@/lib/apollo/errors`) with a single generic fallback. It has **no** FORBIDDEN-specific banner and no UNAUTHENTICATED-specific branch — the page-level redirect already handled the non-owner case before this component mounted (the cardgroup wrapper `CardgroupBatchImportForm` in `frontend/src/components/cardgroups/cardgroup-batch-import-form.tsx` supplies the owner-scoped `importCards` mutation). Adding a FORBIDDEN banner here would be dead code, and lobbying for the backend to emit FORBIDDEN to "activate" it would undo the anti-enumeration design.

## Related rules

- [`UNAUTHENTICATED` collapses to generic copy; `FORBIDDEN` preserves the server's specific reason](unauthenticated-vs-forbidden-message-asymmetry.md) — the inverse case where FORBIDDEN's message IS actionable.
- [Initial-load UNAUTHENTICATED redirects belong in the RSC `page.tsx`, not in the client `error.tsx`](initial-load-unauthenticated-redirects-in-rsc.md) — where the redirect that gates the owner-only UI lives.
