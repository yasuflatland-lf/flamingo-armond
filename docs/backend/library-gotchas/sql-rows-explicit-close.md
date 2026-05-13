# `*sql.Rows`: explicit `rows.Close()` after loop in addition to `defer rows.Close()`

> Part of the [Go library gotchas](./../../../.claude/rules/go-library-gotchas.md) rules.

## Pattern

After iterating `*sql.Rows`, call `rows.Close()` explicitly after the `rows.Err()` check, in addition to the safety-net `defer rows.Close()`.

```go
rows, err := db.QueryContext(ctx, `SELECT id, email FROM auth.users WHERE id = ANY($1)`, pq.Array(ids))
if err != nil {
    return eris.Wrap(err, "query users")
}
defer rows.Close() // safety net if an early return fires

for rows.Next() {
    var id, email string
    if err := rows.Scan(&id, &email); err != nil {
        return eris.Wrap(err, "scan user row")
    }
    // process row
}
if err := rows.Err(); err != nil {
    return eris.Wrap(err, "rows iteration")
}
rows.Close() // release cursor immediately; double-close is safe
```

## Why two closes

`defer rows.Close()` holds the server-side cursor open until the enclosing function returns. When the function performs non-trivial work after the iteration loop — JSON marshaling, file I/O, additional DB queries — all cursors deferred until that point remain open and occupy connection-pool slots. Under connection-pool pressure, a second query inside the same function can deadlock waiting for a slot that the first query's deferred close has not yet released.

Calling `rows.Close()` explicitly after `rows.Err()` releases the cursor and the connection slot immediately, before the post-iteration work begins.

## Double-close is safe

`(*sql.Rows).Close()` is idempotent per the `database/sql` contract. The second call (from `defer`) is a no-op. There is no need to track whether the explicit close already ran.

## The safety net `defer` is still required

The explicit close only fires on the straight-line path through the loop. If `rows.Scan` returns an error and the function returns early, the cursor is still open. The `defer rows.Close()` at the top covers every early-return path and must remain in place.
