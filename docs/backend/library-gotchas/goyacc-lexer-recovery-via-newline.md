# goyacc lexer: recover via NEWLINE for `error NEWLINE`; use explicit skip productions for lone rows

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## What

`goyacc` grammars commonly include a line-level recovery rule such as:

```yacc
entries: error NEWLINE { /* discard bad line */ }
```

`error NEWLINE` is the general goyacc hook for any parse-error recovery — the LALR stack enters error mode whenever the parser encounters a token it cannot shift or reduce in the current state. That includes both parser-level syntax errors (unexpected token sequence) and lexer-level unrecognised characters (the lexer emits the special `error` token after calling `yyerror`). The `yyerrflag = 3` counter decrements with each accepted token; the parser stays silent until it counts down to zero, so multiple tokens on the same bad line are consumed without redundant error messages.

In the `textdic` grammar, all parser-level "expected this, got that" cases the design cares about — a lone WORD without a paired DEFINITION, and a lone DEFINITION without a preceding WORD — are handled by explicit skip productions:

```yacc
entry: WORD { /* record skip */ }
     | DEFINITION { /* record skip */ }
```

Those productions match cleanly at the token level and emit a validation message before continuing. Because they are explicit alternatives, the parser never enters error-recovery mode for them. As a result, `error NEWLINE` in this grammar now only runs for lexer-level unrecognised characters: input that the lexer cannot tokenise at all and must skip to the next line terminator.

The distinction matters: explicit skip productions are for expected-but-unwanted rows; `error NEWLINE` is the last-resort recovery for input the parser cannot make sense of even after the lexer has done its best.

For the Notion sync behaviour that depends on this grammar design, see [`docs/notion-sync.md` § "Behavior"](../../notion-sync.md#behavior).

## Why `error NEWLINE`

Emitting `NEWLINE` after consuming up to and including the next line terminator lets the parser's existing `error NEWLINE` recovery fire. Only the bad line is discarded; subsequent lines parse normally and their `ValidationError` entries accumulate alongside any well-formed results.

Returning `0` (EOF) from the lexer's error path instead aborts the whole parse — every line after the first bad rune is dropped. That is the wrong default for a batch parser that should surface as many diagnostics as possible in one pass.

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
