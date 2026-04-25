# Language policy

All committed text in this repo must be **English only**. This applies to:

- Source comments and docstrings (`backend/**/*.go`, `frontend/src/**/*.{ts,tsx}`, `schema/**/*.graphql`, SQL, YAML, etc.).
- Documentation under `docs/` and any `README.md`.
- Commit messages, PR titles, and PR descriptions.
- Identifiers in code (variables, functions, types, files).

Chat replies between Claude Code and the user remain in 日本語 — the rule covers what gets committed, not the live conversation. When translating existing Japanese text, preserve technical terms and code identifiers as-is; only natural-language prose is translated.

## No PR-order references

Do not reference PR numbers (`PR6`, `PR11`, …) or merge order in committed text. PR numbers are unstable across squash/rebase/fork-merge and rot quickly. Phrase architectural facts as standing statements ("the Supabase SSR client uses…") rather than historical notes ("PR6 introduces…"). Issue links (`[#22]`) are permanent and may stay.

## Verification

Run before committing — both should print nothing:

```bash
# No CJK characters in tracked source/docs.
grep -rlP "[\x{3040}-\x{30ff}\x{4e00}-\x{9fff}]" \
  --include="*.go" --include="*.ts" --include="*.tsx" \
  --include="*.graphql" --include="*.sql" --include="*.md" \
  docs backend/internal backend/cmd backend/graph frontend/src

# No PR-order references.
grep -rnE "PR[0-9]+" docs backend/internal backend/cmd backend/graph frontend/src
```
