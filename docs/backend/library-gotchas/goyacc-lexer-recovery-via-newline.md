# goyacc lexer: recover via NEWLINE for `error NEWLINE`; use explicit skip productions for lone rows

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## What

`goyacc` grammars commonly include a line-level recovery rule such as:

```yacc
entries: error NEWLINE { /* discard bad line */ }
```

This rule is only about lexer-level failures. It is reached when the lexer emits `NEWLINE` after skipping the bad input. Returning `0` (EOF) from the lexer's error path aborts the whole parse — every line after the first bad rune is dropped.

If a grammar wants to ignore a structurally valid lone front-only or back-only row, model that with explicit productions such as:

```yacc
entry: WORD { /* record skip */ }
     | DEFINITION { /* record skip */ }
```

That kind of skip is a parser decision, not recovery from bad input. It should emit a validation message and continue without relying on `error NEWLINE`.

## Why

Emitting `NEWLINE` after consuming up to and including the next line terminator lets the parser's existing `error NEWLINE` recovery fire. Only the bad line is discarded; subsequent lines parse normally and their `ValidationError` entries accumulate alongside any well-formed results.

Explicit skip productions should not use the recovery path. They are for expected-but-unwanted rows that still parse cleanly at the token level.

## Pattern

Extract the recovery into a small helper that reads forward until the next line terminator, increments the line counter consistent with the regular newline path, and returns `NEWLINE` — or `0` only when EOF arrives before a trailing newline:

```go
func (l *lexer) recoverToNewline() int {
    for {
        r, _, err := l.input.ReadRune()
        if err != nil {
            if err != io.EOF {
                l.Error("read: " + err.Error())
            }
            return 0
        }
        if l.isNewLine(r) {
            // consume \n of a \r\n pair explicitly (see CRLF note below)
            if r == '\r' {
                l.input.ReadRune() //nolint:errcheck
            }
            l.tokenLine = l.lineNo
            l.lineNo++
            return NEWLINE
        }
    }
}
```

Call this helper from the lexer's unrecognised-rune branch instead of returning `0`:

```go
l.Error(fmt.Sprintf("unrecognized character %q", r))
return l.recoverToNewline()
```

## CRLF subtlety

If `isNewLine('\r')` peeks at the next byte to detect a `\r\n` pair but does not consume it, the recovery helper must read the `\n` half explicitly. Otherwise the next `Lex` call sees a stray `\n`, emits a spurious `NEWLINE` token, and shifts subsequent line numbers by one.

## Partial nodes on EOF

When EOF arrives inside the recovery helper without a preceding line terminator, the helper returns `0`. Any nodes the parser assembled before the error are preserved in the grammar's action stack — they are not discarded. Callers should document whether partial results are surfaced or suppressed.
