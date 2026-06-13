#!/usr/bin/env bash
# PostToolUse guard: mirror backend CI grep gates at edit time.
# Operates on the single edited file. Exit 2 surfaces violations to Claude so it
# self-corrects; the file write has already happened, so this re-prompts rather
# than blocking. Source rules: .claude/rules/error-wrapping.md, language-policy.md.
#
# NOTE: CI also runs an awk-based "Forbid resolver returning raw usecase error"
# gate over graph/resolver/*.resolvers.go (backend.yml). That check is multi-line
# and stateful (it scans a 5-line window after each usecase call), so a per-file
# single-window port would misread context. It is deliberately left to CI only.
set -euo pipefail
FILE="$(jq -r '.tool_input.file_path // empty')"
[ -z "$FILE" ] && exit 0
case "$FILE" in
  */backend/internal/*.go|*/backend/cmd/*.go|*/backend/graph/*.go) ;;
  *) exit 0 ;;
esac
case "$FILE" in *_test.go) exit 0 ;; esac

violations=""
add() { violations="${violations}$1"$'\n'; }

if grep -nE 'fmt\.Errorf\([^)]*%w' "$FILE" >/dev/null 2>&1; then
  add "eris violation: fmt.Errorf(\"...%w\") is forbidden; use eris.Wrap. See .claude/rules/error-wrapping.md."
fi
case "$FILE" in
  *internal/usecase/ucerr/errors.go) ;;
  *) if grep -nE '&ucerr\.(ValidationError|ForbiddenError)\{' "$FILE" >/dev/null 2>&1; then
       add "ucerr violation: use ucerr.NewValidationError / ucerr.NewForbiddenError, not struct literals. See .claude/rules/error-wrapping.md."
     fi ;;
esac
case "$FILE" in
  *internal/gqlerr/*) ;;
  *) if grep -nE '"code"[[:space:]]*:[[:space:]]*"(UNAUTHENTICATED|BAD_USER_INPUT|FORBIDDEN|INTERNAL|CANCELLED)"' "$FILE" >/dev/null 2>&1; then
       add "wire-format violation: hardcoded extensions.code outside gqlerr. Use gqlerr.* / gqlerr.FromUsecaseError. See .claude/rules/error-wrapping.md."
     fi ;;
esac
# print-if + non-empty test is the canonical CJK check (see .claude/rules/language-policy.md).
# Do NOT use `perl -ne 'exit 0 if //; END { exit 1 }'`: an END block runs even after an
# explicit `exit 0` and overrides the status to 1, so that form never detects anything.
if [ -n "$(perl -CSD -ne 'print if /[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]/' "$FILE")" ]; then
  add "language-policy violation: CJK characters in committed source. English only. See .claude/rules/language-policy.md."
fi

if [ -n "$violations" ]; then
  printf '%s\n%s' "backend-convention-guard found issues in $FILE:" "$violations" >&2
  exit 2
fi
exit 0
