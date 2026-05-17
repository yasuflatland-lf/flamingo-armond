# Observability

> The backend ↔ frontend contract for tracing, request-ID propagation, and Automatic Persisted Queries. Implementation notes that are sided (backend-only or frontend-only) live in [`docs/backend.md` § "Observability"](backend.md#observability) and [`docs/frontend/observability.md`](frontend/observability.md) respectively.

## Tracing (OpenTelemetry)

Tracing is OpenTelemetry-based. The backend emits spans for every GraphQL operation, resolver, and scalar field; spans are exported via OTLP HTTP (port `4318`) to the collector pointed to by `OTEL_EXPORTER_OTLP_ENDPOINT`. When that variable is empty or missing, a no-op TracerProvider is installed and startup logs `telemetry disabled` — dev and CI do not require a running collector.

Per GraphQL request, the span model is:

- One **operation-level** span named after the GraphQL operation (`Me`, `UpdateProfile`, `Health`).
- One **resolver** span per top-level resolver (`Query.me`, `Mutation.updateProfile`).
- One **field** span per non-trivial field resolver (`User.displayName`, `User.bio`, `User.avatarUrl`).
- Span attributes include `graphql.operation.type`, `graphql.operation.name`, and `graphql.error.code` (when the operation errors).

Sampling is `ParentBased(TraceIDRatioBased(ratio))`. The `ratio` is read from `OTEL_TRACES_SAMPLER_ARG` (default `1.0`, i.e. sample everything). In production, set this to a low value (e.g. `0.1`). `ParentBased` means that if a caller provides a sampling decision via `traceparent`, the backend respects it.

The frontend does not yet forward `traceparent`, so every backend request is currently its own root trace. End-to-end OTel propagation from Next.js → Go is tracked in [#22](https://github.com/yasuflatland-lf/flamingo-armond/issues/22) and will introduce a parent span the backend nests under. Until then, the cross-side correlation handle is `X-Request-ID` (see below), not `traceparent`.

## Request ID propagation

Every HTTP request carries an `X-Request-ID` header. The header provides a cheap, grep-friendly correlation ID that flows from the frontend through the backend without requiring a full distributed-tracing stack. It complements, rather than replaces, the OTel span tree tracked in [#22](https://github.com/yasuflatland-lf/flamingo-armond/issues/22).

**Wire contract:**

- Header name: `X-Request-ID` (case-insensitive on read).
- Format: UUID v7 — timestamp-prefixed, lexicographically sortable. The 48 high bits encode the Unix timestamp in milliseconds, making IDs traceable to their creation time.
- Length cap: an upstream value is accepted as-is when it is non-empty and **at most 128 characters**. Values longer than that are dropped and replaced by a freshly generated ID. The cap prevents log injection and guards against accidentally forwarding an unbounded upstream payload into structured log fields.

**Behavioural contract:**

- The frontend generates an `X-Request-ID` on every outgoing GraphQL request (browser Apollo chain and RSC `gqlFetch`).
- The backend echoes the header on the response and stamps `request_id` on **every** structured log line emitted within that request's context. Every `slog.*Context` call (`slog.InfoContext`, `slog.ErrorContext`, etc.) automatically includes `request_id`; resolvers and usecases do not pass it manually.
- A single grep on `request_id` therefore spans the full frontend-to-backend call.

`slog` calls made **without** a context (`slog.Info(...)`) do not carry a request ID — that is by design; they represent process-level events rather than per-request ones.

## Automatic Persisted Queries

APQ shrinks GraphQL POST bodies by replacing the query document with a sha256 hash that the backend resolves against an LRU cache.

**Wire contract:**

- Hash algorithm: `sha256` over the printed query document, hex-encoded.
- Envelope: `extensions.persistedQuery.{ version: 1, sha256Hash }` on the request body.
- Transport: **POST only**. We deliberately do not switch hash-only requests to GET (`useGETForHashedQueries: false`) because the Supabase access token travels in the `Authorization` header; moving to query-param auth would leak the token into server access logs.
- Fallback: if the backend has not seen the hash, it returns a GraphQL error with `extensions.code === "PERSISTED_QUERY_NOT_FOUND"`. The client retries the POST with `query: <full document>` + the same `extensions` payload, which populates the backend's LRU cache for subsequent requests.

**Behavioural contract:**

- Hash-less queries (`{ query: "...", variables: ... }` with no `extensions.persistedQuery`) still work as normal POST bodies, so dev / playground flows are unaffected.
- The browser Apollo chain runs APQ between auth and HTTP so hash-only POST bodies still carry the `Authorization: Bearer` header.
- **RSC opt-out**: the server-side `gqlFetch` always sends the full printed document — no APQ negotiation. RSC requests are infrequent compared to the browser, and the APQ retry loop would double the code surface of `gqlFetch`. The recipe to opt RSC in later: compute `sha256Hex(print(doc))`, POST with the `extensions.persistedQuery` envelope, and on `PERSISTED_QUERY_NOT_FOUND` retry with `query: print(doc)` + the same `extensions`.
