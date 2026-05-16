# Docblock correction cascades — audit rule prose + bullet list on the same edit

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## Why

A production docblock and its mirrored description in `.claude/rules/` or `docs/` start out
identical. When the docblock is corrected — to fix a factually wrong claim, tighten a contract,
or reflect a changed implementation — the mirrored text is easy to overlook. The result is an
inconsistency that persists silently until a reader notices the contradiction between the source
file and the rule file.

The same cascade can affect two distinct surfaces:

1. **Rule prose** — a sentence in a `.claude/rules/*.md` file that restates the docblock's claim.
2. **Bullet / recommendation lists** — a code snippet or `- [Type]` line in a nearby list that
   showed the old API as the recommended pattern.

Both surfaces must be updated on the same edit as the docblock correction.

## How to apply

After correcting any production docblock:

1. **Grep `.claude/rules/` and `docs/` for the symbol name and the prior wording.**

   ```bash
   grep -rn "NewValidationError\|value form\|silently falls through" \
       .claude/rules/ docs/
   ```

2. **Update rule prose** that restates the old claim.

3. **Update bullet lists** that show the old API shape as a positive example.  
   A bullet reading `&ucerr.ValidationError{Field, Message}` (struct-literal form) contradicts a
   corrected constructor mandate — it is a recommendation list entry, not just a description.

4. **Commit all three edits together** (source file, rule prose, bullet list). A two-step process
   where the rule file is updated "in a follow-on commit" means the inconsistency ships in the
   meantime.

## Worked example

In the `ucerr.NewValidationError` correction ([#160]):

- The docblock was corrected from "value form silently falls through" to "the compiler rejects
  the value form".
- `.claude/rules/error-wrapping.md` still said "silently falls through" — required a follow-on
  commit to align.
- A bullet list nearby still showed `&ucerr.ValidationError{Field, Message}` (the old
  struct-literal form), now contradicting the constructor mandate — required a second follow-on
  commit.

All three surfaces should have been updated in the original correction commit.

## Scope

This rule applies any time a docblock correction changes:

- A behavioral claim (e.g., "panics" vs "silently falls through" vs "returns an error").
- An API shape (constructor vs struct literal, pointer vs value receiver).
- A contract boundary (which argument is optional, which triggers a panic).

It does **not** require a full audit of the entire rule file on every doc edit — only a targeted
grep for the corrected symbol and its prior wording.
