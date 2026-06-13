# Drop the backend not-found message at the client boundary

> Part of [`.claude/rules/frontend-rsc-error-handling.md`](../../../.claude/rules/frontend-rsc-error-handling.md). The frontend complement to the backend's [non-disclosure gate](../../backend/ddd-patterns/notfound-collapse-non-disclosure.md).

When the backend collapses "unknown id" and "exists but hidden" into one typed not-found error to avoid being an existence oracle (the non-disclosure gate), the client must not undo that protection — and must not let a future contributor undo it either.

## The rule

A typed not-found error reaching the client carries a deliberately state-free `message`. The client:

1. **Renders its own localized copy**, never the server `message`. The server string could later drift to disclose state ("draft", "deleted by owner") or echo user input, and it is not localized.
2. **Does not thread the server `message` into the discriminated outcome.** The hook's outcome variant for the not-found case carries **no payload** (`{ status: "not_found" }`, not `{ status: "not_found"; message: string }`). A carried-but-unread field is dead data that invites a future contributor to render it — re-opening the disclosure/locale/PII hole the backend closed. Dropping the field makes "render the backend message" *unrepresentable*, which is a stronger guarantee than a comment asking the next reader not to.

Discard the message at the conversion boundary (the `useMutation`/`gqlFetch` wrapper), logging nothing user-derived — the not-found case is already surfaced to the user via the localized banner, so no triage warn is needed.

## Worked example

`frontend/src/app/catalog/use-import-master.ts`: `importMasterCardgroup` returns a `MasterNotFoundError` (the backend collapses unknown-id and unpublished-draft into one error). The hook maps it to `{ status: "not_found" }` — the `MasterNotFoundError.message` is read nowhere — and the catalog client renders `t("importNotFound")`. The GraphQL selection may still over-fetch `message` (a harmless one-string over-read that keeps the union shape explicit in the document); the load-bearing guarantee is that no code path reads it.

## Boundary

This applies only across the trust boundary the backend protects. An **owner-facing** read of the same resource (where disclosing existence is intentional) may surface a specific reason — keep the split where the two codes diverge in user-facing copy (see [`unauthenticated-vs-forbidden-message-asymmetry.md`](unauthenticated-vs-forbidden-message-asymmetry.md)).
