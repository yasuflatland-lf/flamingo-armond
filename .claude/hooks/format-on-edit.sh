#!/usr/bin/env bash
# PostToolUse formatter: auto-format the single edited file so Biome / gofmt
# diffs never surprise the developer in CI. Exit 0 always — a formatting failure
# must never block the session.
set -euo pipefail
FILE="$(jq -r '.tool_input.file_path // empty')"
[ -z "$FILE" ] && exit 0
case "$FILE" in
  *.ts|*.tsx|*.js|*.mjs)
    # MUST be `biome check --write`, not `biome format --write`:
    # only `check` applies assist/source/organizeImports (see .claude/rules/subagent-dispatch.md).
    (cd "$CLAUDE_PROJECT_DIR/frontend" && pnpm exec biome check --write "$FILE" >/dev/null 2>&1) || true
    ;;
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$FILE" || true
    ;;
esac
exit 0
