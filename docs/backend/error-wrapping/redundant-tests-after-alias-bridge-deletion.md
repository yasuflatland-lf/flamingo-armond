# Redundant tests after alias-bridge deletion — cross-check existing table cases before retaining

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When an alias bridge (e.g. `usecase/errors.go`) is deleted because the import
cycle that motivated it is gone, the tests written to prove alias transparency
become orphaned. Those tests typically assert that `usecase.X` resolves to the
same classification as `ucerr.X`. Once the alias no longer exists the invariant
they were testing no longer exists either; what remains is a plain
classification check.

**Rule:** After deleting an alias bridge, identify every test written to prove
alias transparency. Check whether an existing table-driven test already covers
the same input/assertion pair. If yes, delete the standalone tests. Do not
retain them as direct-classification duplicates — they hide each other's
failures and inflate line count without adding coverage.

## Worked example ([#160])

`gqlerr/from_usecase_test.go` contained two standalone tests:

- `TestValidationErrorAliasIsTransparent` — proved `usecase.ValidationError`
  was classified identically to `ucerr.ValidationError`.
- `TestErrUnauthenticatedReexportIsTransparent` — proved `usecase.ErrUnauthenticated`
  re-exported the same sentinel as `ucerr.ErrUnauthenticated`.

When `usecase/errors.go` was deleted (the alias bridge), both tests were
rewritten as `TestValidationErrorClassification` and
`TestErrUnauthenticatedClassification` — renaming "alias transparency" to
"direct classification". A simplifier pass then found that both new tests were
exact duplicates of cases already present in the `TestFromUsecaseError`
table. The residual classification path was already exercised there; the
standalone tests were dead weight. Removing them eliminated 22 lines.

## How to check

After rewriting alias-transparency tests as direct-classification tests:

1. List every input type and expected `extensions.code` value the rewritten
   test asserts.
2. Grep the table-driven test for the same input type:
   ```bash
   grep -n 'ValidationError\|ErrUnauthenticated' \
     backend/internal/gqlerr/from_usecase_test.go
   ```
3. If every case appears in the table, delete the standalone test function.
4. Run the full test suite to confirm coverage is unchanged:
   ```bash
   go test -v -race ./backend/internal/gqlerr/...
   ```

The key signal is that the alias bridge is the only thing that made the
standalone test non-redundant. Without the bridge, "alias is transparent"
reduces to "type classifies correctly" — which the table already asserts.

[#160]: https://github.com/yasuflatland-lf/flamingo-armond/issues/160
