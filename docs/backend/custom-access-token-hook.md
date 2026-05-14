# Supabase custom access token hook

> Part of [`docs/backend-db.md`](../backend-db.md) and [`docs/backend-auth.md`](../backend-auth.md). Covers the design of `public.custom_access_token_hook` — the Postgres function GoTrue invokes at JWT mint time to inject `app_metadata.role` into the access token. The frontend reads the claim via `supabase.auth.getClaims()` and skips a GraphQL round-trip for header role gating. See [`docs/frontend/auth-supabase.md`](../frontend/auth-supabase.md) for the consumer side.

The function lives at `backend/internal/database/migrations/20260514000000_add_custom_access_token_hook.up.sql` and is wired in `supabase/config.toml` under `[auth.hook.custom_access_token]`. Each subsection below isolates one design decision that is non-obvious from the Supabase docs.

## Join-at-mint over sync-trigger for role claims

The function computes `app_metadata.role` by joining `public.user_roles` JOIN `public.roles` each time GoTrue issues an access token. The obvious alternative — a sync trigger on `user_roles` that mirrors the role into `auth.users.raw_app_meta_data` — duplicates the source of truth and silently drifts whenever the trigger is bypassed: manual SQL, `pg_restore` of a logical backup, schema change that disables the trigger, or any future migration that touches the same column.

The mint-time hook recomputes from `public.user_roles` on every token rotation, so a revoked admin loses the claim at the next token refresh (≤ `jwt_expiry`) without operator intervention. The hook is the only path that produces the token-side `role` claim; there is no parallel sync surface to keep in lockstep.

## Fail closed on malformed events

A Supabase Custom Access Token Hook receives `event jsonb` shaped `{ "user_id": uuid, "claims": jsonb, ... }`. Three pitfalls combine to make a quiet failure plausible:

- `event->>'user_id'` returns SQL `NULL` when the key is absent.
- `NULL::uuid` cast succeeds silently (it does not raise).
- `WHERE ur.user_id = NULL` matches zero rows under three-valued logic.

Without an explicit guard, a malformed payload mints a token where the role claim is stripped because the lookup matched nothing — the same observable result as a non-admin user — but the token is otherwise valid and signed. The hook therefore checks both keys at entry and raises:

```sql
IF event->>'user_id' IS NULL OR event->'claims' IS NULL THEN
    RAISE LOG 'custom_access_token_hook: rejecting malformed event (user_id present=%, claims present=%)',
        (event ? 'user_id'), (event ? 'claims');
    RAISE EXCEPTION 'custom_access_token_hook: malformed event (user_id and claims are required)';
END IF;
```

`RAISE LOG` runs before `RAISE EXCEPTION` deliberately: the LOG line lands in the Postgres server log independent of client error propagation, so an operator triaging a failed token mint can correlate the GoTrue HTTP error with the PG-side reason even if the GoTrue trace was lost. The LOG line carries only key-presence booleans — no claim contents, no `user_id` — to avoid PII leakage through the server log.

## Stale claim removal on revocation

When the user is **not** admin, the hook must explicitly strip the `role` key from the returned `app_metadata`, not just leave it untouched:

```sql
IF is_admin_user THEN
    app_metadata := jsonb_set(app_metadata, '{role}', '"admin"'::jsonb, true);
ELSE
    -- Clear any stale role claim so a revoked admin does not keep a stale
    -- token-side metadata entry on refresh.
    app_metadata := app_metadata - 'role';
END IF;
```

A previously-admin user whose `user_roles` row is deleted still has a token in their cookie carrying `app_metadata.role = "admin"`. On the **next** mint (sign-in or refresh), the hook recomputes membership and writes the result back into `app_metadata`. Without the `- 'role'` branch, the hook leaves the inbound `app_metadata` (with the stale claim) untouched and returns it as-is, extending the security-relevant TTL of the revocation by however long the user's session lasts before the next manual sign-out.

The strip is unconditional on the non-admin branch — there is no other code path that writes `app_metadata.role`, and a future second privileged role will go through the same `IF / ELSE` shape.

## Canonical return shape `jsonb_build_object('claims', new_claims)`

GoTrue is documented to merge the function's returned `claims` key into the access token. Two return shapes both work today:

```sql
-- Canonical (matches every Supabase doc / sample).
RETURN jsonb_build_object('claims', jsonb_set(original_claims, '{app_metadata}', app_metadata, true));

-- Works today but diverges from the contract.
RETURN jsonb_set(event, '{claims}', jsonb_set(original_claims, '{app_metadata}', app_metadata, true), true);
```

The second form echoes the full event back. GoTrue currently ignores keys other than `claims`, but the documented contract is the first form, and a future GoTrue release that becomes strict about the returned shape would silently fail every token mint. The canonical form is one line of code; align with it.

## DROP FUNCTION auto-revokes privileges

Down migrations for hook functions need only `DROP FUNCTION IF EXISTS ...`. The REVOKE / GRANT pairs in the up migration do not need explicit reversal:

```sql
-- Down migration is sufficient.
DROP FUNCTION IF EXISTS public.custom_access_token_hook(jsonb);
```

PostgreSQL automatically discards all privileges granted on the function object when the function is dropped — there is no orphaned `aclitem` row to clean up. Document this fact in the down migration's header comment so a future maintainer does not add a redundant `REVOKE EXECUTE ... FROM supabase_auth_admin` block above the `DROP`, which would itself fail under the same `IF EXISTS (SELECT 1 FROM pg_roles ...)` portability guard the up migration uses.

## Operator precondition: disable the hook in config before dropping the function

Dropping the function while `[auth.hook.custom_access_token] enabled = true` in `supabase/config.toml` (or the equivalent dashboard setting) causes every subsequent token mint to fail with `function public.custom_access_token_hook(jsonb) does not exist`. The down migration cannot order this for the operator — disabling the hook is a config change that lives outside the Postgres migration history.

The down migration's header must therefore carry a standing operator instruction:

```sql
-- Operator precondition: before applying this migration, disable the hook in
-- supabase/config.toml ([auth.hook.custom_access_token] enabled = false) and
-- redeploy the auth service. Dropping the function while the hook is still
-- enabled will fail every subsequent token mint.
```

Phrase the instruction as a standing rule, not a historical note about the migration's introduction — the reader who arrives during a rollback months later needs the precondition first, not provenance.
