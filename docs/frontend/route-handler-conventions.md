# Route Handler conventions

> Part of [`docs/frontend.md`](../frontend.md). See the index for related chapters.

Route Handlers under `frontend/src/app/api/**/route.ts` follow the same per-route layout as pages:

- `route.ts` for the handler.
- `queries.ts` (sibling) for any `graphql()` tagged template the handler uses. Mirror the page-level convention (`app/cardgroups/queries.ts`, `app/_components/queries.ts`); do **not** inline the document into `route.ts`. Shared queries live next to their consumer, not in a global `lib/` bag.
- `route.test.ts` (sibling) for unit coverage.

### Discriminated-union response shape

Route Handlers that can fail typed (auth, validation, upstream-down) return a discriminated union so consumers narrow exhaustively on a tag field:

```ts
type HealthzResponse = { ok: true; backend: string } | { ok: false; error: string };

const body: HealthzResponse = { ok: true, backend: data.health };
return NextResponse.json(body);
```

The annotation on `body` is load-bearing — `NextResponse.json(...)` is generic over `unknown`, so without the explicit type the compiler accepts any object shape and a refactor that drops `ok` from one branch passes typecheck. Reference: `frontend/src/app/api/healthz/route.ts`. The general "discriminated union over flat DTO" rule (covering factory output beyond Route Handler responses) lives in [`.claude/rules/frontend-typescript-conventions.md` § "Discriminated union over flat DTO when consumers must branch on the variant"](../.claude/rules/frontend-typescript-conventions.md#discriminated-union-over-flat-dto-when-consumers-must-branch-on-the-variant).

### `/api/healthz` JSON probe

A public, JSON, cache-bypassing health endpoint. Conventions baked in:

1. **Public** — no auth header required. The backend `health` resolver is whitelisted so no `UNAUTHENTICATED` ever returns. External uptime monitors can poll it without managing credentials.
2. **`{ revalidate: 0 }`** on the upstream `gqlFetch` so a cached HTTP response cannot mask a live backend outage.
3. **Status code, not body**: success is `200 + { ok: true, backend: <string> }`; failure is `503 + { ok: false, error: <string> }`. **Never** return `200` with `{ ok: false, ... }` embedded — generic monitors check status codes, not body parsers, and a 200-with-error silently passes every uptime check while the system is down.
4. **Log before responding on failure**: `console.error("[healthz] backend health check failed:", err)` so an operator can correlate the 503 in logs with the cause.

