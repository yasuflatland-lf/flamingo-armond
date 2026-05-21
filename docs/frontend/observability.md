# Observability

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Tracing model, request-ID contract, and APQ wire format are documented in `docs/observability.md` (single source of truth across backend and frontend). Frontend-only implementation notes follow.

### Request ID generator and propagation

**Generator (`frontend/src/lib/observability/request-id.ts`).** A local UUID v7 generator backed by `globalThis.crypto.getRandomValues`. It stores the last emitted millisecond timestamp so timestamp prefixes are non-decreasing across calls in the same runtime, and it sets the version and variant nibbles explicitly. Produces a standard UUID v7 string:

```
xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx
```

**Web Crypto availability + fallback.** The module checks `globalThis.crypto.getRandomValues` availability **once at module load** — not per call — and emits a `console.warn` if the API is absent. When unavailable (older Safari, certain JSDOM configurations, some Node environments), it falls back to `Math.random`-derived bytes, preserving the UUID v7 format contract at reduced collision resistance. Checking once at load time ensures the warning fires exactly once regardless of call count. The `lastTimestampMs` monotonicity clamp is **unconditional** and must remain outside the `if (_cryptoAvailable)` branch; placing it inside would allow the fallback path to produce non-monotone timestamps, breaking UUID v7's time-ordering property. The test suite asserts this invariant with a 100-call sequence check on the fallback path (`frontend/src/lib/observability/request-id.test.ts`, `Web Crypto fallback path` describe block).

**Browser (Apollo link chain).** `frontend/src/lib/apollo/request-id-link.ts` exports `requestIdLink`, an Apollo `setContext` link. It checks for an existing `X-Request-ID` header case-insensitively; if none is found it generates a fresh UUID v7 and attaches it. The link is prepended as the first link in `makeClient()`:

```ts
from([requestIdLink, authLink, makeApqLink(), httpLink])
```

Being first in the chain ensures the ID is present for every subsequent link and for the outbound HTTP request.

**RSC (`gqlFetch`).** `frontend/src/lib/apollo/server.ts` assigns a fresh UUID v7 inside `gqlFetch` before the `fetch` call. `gqlFetch` is the RSC entrypoint and has no caller-supplied headers, so every call generates a fresh UUID v7 — chained correlation across server-to-server calls is out of scope at this tier.
