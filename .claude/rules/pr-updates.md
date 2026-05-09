# PR updates

> Applies to: any session that rewrites a PR title or body via `gh pr edit`, including the `/pr-update` skill. Cross-cutting workflow rule.

When rewriting PR titles or bodies, always:

1. **Sample 3–5 recent merged PRs** to match house style (heading order, summary length, test-plan format). Use `gh pr list --state merged --limit 5` and read each body before drafting.
2. **Link the related issue** with `Closes #N` (or `Fixes #N` / `Resolves #N`) so the PR closes the issue on merge. If the branch addresses no issue, say so explicitly in the body.
3. **Use `gh pr edit <N> --body-file <path>`**, never HEREDOC or inline `--body "..."` for multi-line content.

## Why `--body-file` over HEREDOC

HEREDOC bodies repeatedly leak literal backslashes (`\n`, `\\\``) into rendered PR bodies because the shell, `gh`, and Markdown each have their own escape rules and the layers compose unpredictably. Writing the body to a file (e.g. `/tmp/pr-body.md`) and passing `--body-file` bypasses every shell-escape layer — what's in the file is what GitHub renders.

```bash
# Correct
cat > /tmp/pr-body.md <<'TEMPLATE'
## Summary
...

Closes #42
TEMPLATE
gh pr edit 123 --title "feat(scope): short title" --body-file /tmp/pr-body.md

# Incorrect — escape bugs leak into rendered body
gh pr edit 123 --body "$(cat <<'EOF'
...
EOF
)"
```

The `'TEMPLATE'` quoting on the heredoc opener is still important for the file-write step (it prevents `$VAR` expansion in the file content), but the failure mode this rule guards against is the second invocation, where the body is piped through another layer of shell quoting on its way to `gh`.
