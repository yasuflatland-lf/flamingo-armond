# Append-only extension of a classification with a secondary, independently-graded source

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Why

When an existing classification (e.g. CEFR level per word) is extended with a
**second source that grades by a different unit**, a naive merge corrupts the
authoritative source's judgments. The CEFR word list merges duplicate keys with
a harder-wins rule (`out[key] = out[key].Harder(level)` in
[`backend/internal/cefr/wordlist.go`](../../../backend/internal/cefr/wordlist.go)).
The secondary source — the Cambridge English Vocabulary Profile — grades by
*sense*, so its headwords are mostly common words that already carry an
authoritative Oxford A1..C1 level. Tagging every EVP headword C2 and feeding it
into the harder-wins merge would silently promote those common words to C2,
overriding Oxford's correct, finer-grained judgments.

## What

Make the second tier **purely additive**: include only keys that are **absent**
from the authoritative set, then tag the survivors with the new tier. The
authoritative source's existing judgments are never touched, so the intersection
of the new tier with the authoritative levels is empty by construction —
`Oxford ∩ C2 = 0`. Assert that invariant with a test rather than trusting the
generation script; the data is compiled in via `go:embed`, so a regression is a
build defect, not a runtime condition. Because the new tier and the
authoritative set are disjoint, the harder-wins merge stays correct: there are
no shared keys for it to arbitrate.

This keeps the secondary source contributing exactly its unique headwords (the
genuinely-advanced vocabulary Oxford omits) without letting its sense-level
grading bleed into the word-level classification it is layered on top of.

## Reference

The concrete provenance, methodology, and the absent-from-Oxford diff that
produces the C2 tier are documented code-locally in
[`backend/internal/cefr/data/README.md`](../../../backend/internal/cefr/data/README.md)
(§ "Methodology"). The merge is in
[`backend/internal/cefr/wordlist.go`](../../../backend/internal/cefr/wordlist.go)
(`NewWordList` / `ParseMarkdown`, both using `CEFRLevel.Harder`).
