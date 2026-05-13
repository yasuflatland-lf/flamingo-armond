# Named return `(retErr error)` for deferred `tx.Rollback` — local `err` can be shadowed

> Part of the [Go library gotchas](./../../../.claude/rules/go-library-gotchas.md) rules.

## Pattern

Functions that defer `tx.Rollback()` must use a named return value `(retErr error)`, not a local variable `err`.

```go
// CORRECT
func doWork(db *sql.DB) (retErr error) {
    tx, err := db.Begin()
    if err != nil {
        return eris.Wrap(err, "begin tx")
    }
    defer func() {
        if retErr != nil {
            if rbErr := tx.Rollback(); rbErr != nil {
                log.Printf("tx.Rollback failed: %v (original: %v)", rbErr, retErr)
            }
        }
    }()

    if err := someQuery(tx); err != nil {
        return eris.Wrap(err, "some query") // sets retErr, defer sees it
    }

    return tx.Commit()
}
```

```go
// FRAGILE — do not use
func doWork(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if err != nil { // captures the *outer* err
            _ = tx.Rollback()
        }
    }()

    if err := someQuery(tx); err != nil { // shadows outer err with :=
        return err // outer err is still nil; defer rolls back nothing
    }

    return tx.Commit()
}
```

## Why local `err` is fragile

A `defer` closure captures variables by reference. When the deferred function checks `err != nil`, it reads the variable from the enclosing scope at the moment the defer runs — which is after the function returns.

The fragile pattern breaks when a `:=` inside a nested block introduces a new `err` variable that shadows the outer one:

```go
if err := someQuery(tx); err != nil {
    return err
}
```

The `err` on the left of `:=` is a new variable. The outer `err` (captured by the defer) remains `nil`. When `someQuery` fails and the function returns, the defer sees `err == nil` and skips the rollback, leaving the transaction open until the connection is recycled.

Named returns cannot be shadowed in this way. A `return err` inside a nested block assigns to the named return value (`retErr`), which the deferred closure reads correctly.

## Log rollback errors, do not swallow them

A failed rollback during a network partition leaves the database in an ambiguous state. Log the rollback error as a side channel so the operator knows both the original failure and the rollback failure:

```go
defer func() {
    if retErr != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            log.Printf("tx.Rollback failed: %v (original error: %v)", rbErr, retErr)
        }
    }
}()
```

Using `_ = tx.Rollback()` silently discards this diagnostic information.
