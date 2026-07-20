# goyacc lexer: recover by emitting NEWLINE, not `0`; use explicit skip productions for lone rows

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## What

`goyacc` grammars commonly include a line-level recovery rule such as:

```yacc
entries: error NEWLINE { /* discard bad line */ }
```

`error NEWLINE` is the general goyacc hook for any parse-error recovery — the LALR stack enters error mode whenever the parser encounters a token it cannot shift or reduce in the current state. The `yyerrflag = 3` counter decrements with each accepted token; the parser stays silent until it counts down to zero, so multiple tokens on the same bad line are consumed without redundant error messages.

In the `textdic` grammar, all parser-level "expected this, got that" cases the design cares about — a lone WORD without a paired DEFINITION, and a lone DEFINITION without a preceding WORD — are handled by explicit skip productions:

```yacc
entry: WORD { /* record skip */ }
     | DEFINITION { /* record skip */ }
```

Those productions match cleanly at the token level and emit a validation message before continuing. Because they are explicit alternatives, the parser never enters error-recovery mode for them.

The distinction matters: explicit skip productions are for expected-but-unwanted rows; `error NEWLINE` is the last-resort recovery for input the parser cannot make sense of even after the lexer has done its best. In `textdic` the lexer-level unrecognised-character path does not reach it either — see [Why the lexer emits NEWLINE](#why-the-lexer-emits-newline) — but the rule stays in the grammar as the standing hook for any token sequence with no explicit production.

For the Notion sync behaviour that depends on this grammar design, see [`docs/notion-sync.md` § "Behavior"](../../notion-sync.md#behavior).

## Why the lexer emits NEWLINE

When the lexer meets a rune it cannot tokenise it still has to return *some* token. Returning `0` (EOF) aborts the whole parse — every line after the first bad rune is dropped. That is the wrong default for a batch parser that should surface as many diagnostics as possible in one pass.

`textdic` instead consumes the rest of the malformed line and returns `NEWLINE`. Only the bad line is discarded; subsequent lines parse normally and their `ValidationError` entries accumulate alongside any well-formed results.

Note what does *not* happen: the emitted `NEWLINE` does not put the parser into error mode. `backend/internal/textdic/grammar.y:74` declares a blank-line production, `entry: NEWLINE { $$ = node{} }`, so `NEWLINE` is shiftable in the state the parser is in and reduces to an empty entry. Recovery works because the *lexer* has already swallowed the malformed line, not because `error NEWLINE` at `grammar.y:67` fires. Do not reason about this path as though it exercised the error rule: dropping or narrowing the blank-line production would change which mechanism recovers it, and a test that asserts "error recovery fired" here would be asserting the wrong thing.

## Pattern

Extract the recovery into a small helper that reads forward to the end of the malformed line, returns the consumed text so the caller can attach it to the diagnostic, and returns `NEWLINE` on **both** exits — line terminator found, and EOF reached before one:

```go
func (l *lexer) recoverLineForUnrecognized(first rune) (string, int) {
    var b strings.Builder
    b.WriteRune(first)
    for {
        r, _, err := l.input.ReadRune()
        if err != nil {
            if err != io.EOF {
                l.Error("read: " + err.Error())
            }
            // EOF: do not advance lineNo — there is no subsequent token to
            // attribute, and the next Lex call returns 0 on its own.
            return b.String(), NEWLINE
        }
        if l.isNewLine(r) {
            // consume \n of a \r\n pair explicitly (see CRLF note below)
            if r == '\r' {
                l.input.ReadRune() //nolint:errcheck
            }
            l.tokenLine = l.lineNo
            l.lineNo++
            return b.String(), NEWLINE
        }
        b.WriteRune(r)
    }
}
```

Call it from the lexer's unrecognised-rune branch instead of returning `0`, and record the returned snippet on the structured error rather than folding it into a message string:

```go
snippet, tok := l.recoverLineForUnrecognized(r)
l.errors = append(l.errors, parseError{
    Line:    l.tokenLine,
    Message: fmt.Sprintf("unrecognized character %q", r),
    Kind:    SkipKindUnrecognized,
    Snippet: snippet,
})
return tok
```

Returning `NEWLINE` rather than `0` at EOF matters when the malformed line is the last in the payload and carries no trailing newline: the grammar still receives a token that closes the entry, and the *next* `Lex` call reports EOF by itself. Returning `0` there truncates the parse one entry early.

The live implementation is `backend/internal/textdic/lexer.go:184-219`, called from `lexer.go:119`.

## CRLF subtlety

If `isNewLine('\r')` peeks at the next byte to detect a `\r\n` pair but does not consume it, the recovery helper must read the `\n` half explicitly. Otherwise the next `Lex` call sees a stray `\n`, emits a spurious `NEWLINE` token, and shifts subsequent line numbers by one.

## Partial nodes on EOF

When EOF arrives inside the recovery helper without a preceding line terminator, the helper still returns `NEWLINE` and leaves `lineNo` unchanged; the following `Lex` call is the one that returns `0`. Any nodes the parser assembled before the error are preserved in the grammar's action stack — they are not discarded. Callers should document whether partial results are surfaced or suppressed.
