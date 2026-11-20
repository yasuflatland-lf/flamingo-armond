# goyacc lexer: recover by emitting NEWLINE, not `0`; use explicit skip productions for lone rows

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## What

The `textdic` grammar has no `error` production. Every token the lexer emits (`WORD`, `DEFINITION`, `NEWLINE`) is accepted by one of the `entry` alternatives, so the parser never enters goyacc's error-recovery mode on non-empty input; the only syntax error left is `$end` on a token-less stream. Recovery from malformed text happens one layer down, in the lexer.

In the `textdic` grammar, all parser-level "expected this, got that" cases the design cares about — a lone WORD without a paired DEFINITION, and a lone DEFINITION without a preceding WORD — are handled by explicit skip productions:

```yacc
entry: WORD { /* record skip */ }
     | DEFINITION { /* record skip */ }
```

Those productions match cleanly at the token level and emit a validation message before continuing. Because they are explicit alternatives, the parser never enters error-recovery mode for them.

The distinction matters: explicit skip productions are for expected-but-unwanted rows; a goyacc `error` production is the last-resort recovery for input the parser cannot make sense of even after the lexer has done its best. `textdic` has no such rule: the lexer-level unrecognised-character path never produces a token the grammar cannot shift — see [Why the lexer emits NEWLINE](#why-the-lexer-emits-newline).

For the Notion sync behaviour that depends on this grammar design, see [`docs/notion-sync.md` § "Behavior"](../../notion-sync.md#behavior).

## Why the lexer emits NEWLINE

When the lexer meets a rune it cannot tokenise it still has to return *some* token. Returning `0` (EOF) aborts the whole parse — every line after the first bad rune is dropped. That is the wrong default for a batch parser that should surface as many diagnostics as possible in one pass.

`textdic` instead consumes the rest of the malformed line and returns `NEWLINE`. Only the bad line is discarded; subsequent lines parse normally and their `ValidationError` entries accumulate alongside any well-formed results.

Note what does *not* happen: the emitted `NEWLINE` does not put the parser into error mode. `backend/internal/textdic/grammar.y` declares the `entry: NEWLINE { $$ = node{} }` blank-line production, so `NEWLINE` is shiftable in the state the parser is in and reduces to an empty entry. Recovery works because the *lexer* has already swallowed the malformed line; there is no grammar-level error rule to fire. Dropping or narrowing the blank-line production would turn the emitted `NEWLINE` into a hard syntax error, so keep it.

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

Returning `NEWLINE` rather than `0` at EOF matters when the malformed line is the last in the payload and carries no trailing newline: the grammar still receives a shiftable token for that line, and the *next* `Lex` call reports EOF by itself. Returning `0` there does not lose any entry — `$end` follows `start`, so the parser reduces `entries` and every previously accepted entry survives — but when the malformed line is the *only* content there is nothing to reduce yet, and the parser emits a second, spurious `syntax error: unexpected $end` HARD diagnostic on top of the UNRECOGNIZED one. The `unrecognized-eof` case in `backend/internal/textdic/service_test.go` pins that: the payload `"@broken no nl"` must produce exactly one validation error.

The live implementation is `(*lexer).recoverLineForUnrecognized` in `backend/internal/textdic/lexer.go`, called from the unrecognised-rune branch of `(*lexer).Lex` in the same file.

## CRLF subtlety

`isNewLine('\r')` peeks at the next byte to detect a `\r\n` pair but does not consume it, so every path that turns a `\r` into a line break must read the `\n` half explicitly: the recovery helper and the `NEWLINE` branch of `(*lexer).Lex`, which a blank `\r\n` line reaches through `skipWhiteSpace`. Otherwise the next `Lex` call sees a stray `\n`, emits a spurious `NEWLINE` token, and shifts subsequent line numbers by one. `TestProcess_BlankCRLFLineCountsOnce` in `backend/internal/textdic/service_test.go` pins the blank-line case, and `TestProcess_Property_LinesInRange` checks that every line number stays inside the document for any mix of `\n` and `\r\n`.

## Partial nodes on EOF

When EOF arrives inside the recovery helper without a preceding line terminator, the helper still returns `NEWLINE` and leaves `lineNo` unchanged; the following `Lex` call is the one that returns `0`. Any nodes the parser assembled before the error are preserved in the grammar's action stack — they are not discarded. Callers should document whether partial results are surfaced or suppressed.
