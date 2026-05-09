# GORM `LIKE` / `ILIKE` requires escaping `%`, `_`, `\` in user input

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A search box that runs `WHERE name ILIKE ? || '%'` with a user-supplied string becomes a pattern-injection surface: a user typing `100%` matches every row containing the literal string `100`, not just rows starting with `100%`. The three Postgres `LIKE` metacharacters are `%`, `_`, and `\` (the default escape). User-supplied search text must be escaped before being wrapped with `%...%`:

```go
func escapeLike(s string) string {
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `%`, `\%`)
    s = strings.ReplaceAll(s, `_`, `\_`)
    return s
}
// ...
db.Where("name ILIKE ?", "%"+escapeLike(query)+"%")
```

Order matters: escape `\` first, then `%` and `_`, otherwise the second pass re-escapes the backslash from the first pass. The same rule applies to `name ILIKE ? || '%'` (prefix match) and to any other `LIKE` predicate fed by user input.
