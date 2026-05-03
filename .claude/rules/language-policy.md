# Language policy

All committed text in this repo must be **English only**. This applies to:

- Source comments and docstrings (`backend/**/*.go`, `frontend/src/**/*.{ts,tsx}`, `schema/**/*.graphql`, SQL, YAML, etc.).
- Documentation under `docs/` and any `README.md`.
- Commit messages, PR titles, and PR descriptions.
- Identifiers in code (variables, functions, types, files).

Chat replies between Claude Code and the user remain in 日本語 — the rule covers what gets committed, not the live conversation. When translating existing Japanese text, preserve technical terms and code identifiers as-is; only natural-language prose is translated.

## No PR-order references

Do not reference PR numbers (`PR6`, `PR11`, …) or merge order in committed text. PR numbers are unstable across squash/rebase/fork-merge and rot quickly. Phrase architectural facts as standing statements ("the Supabase SSR client uses…") rather than historical notes ("PR6 introduces…"). Issue links (`[#22]`) are permanent and may stay.

The same rule extends to **self-referential change-history phrasing**: "(already done by the same change that introduces this section)", "(this PR adds…)", "(in this section we now…)" all rot the moment another change touches the file. The reader who arrives six months later has no way to know which "this section" the parenthetical meant. Phrase preconditions as standing operator instructions ("Confirm `render.yaml` declares X. If it does not, land that first.") rather than historical narration. The same review cycle that exposed the PR-number rule also surfaced the self-referential variant; both fail for the same reason.

## Verify cross-doc symbols against the source of truth, never against a sibling doc

When a doc references a GraphQL mutation name, a Go function name, an env-var name, or any other code-level identifier, the source of truth is `schema/schema.graphql`, the Go source file, or the canonical config — **never** a sibling doc. A typo in `docs/A.md` that gets copied verbatim into `docs/B.md` looks self-consistent (both docs agree) and survives review by anyone who checks A and B against each other instead of against the schema. The failure mode that exposed this rule: an `adminRevokeRole` reference (no such mutation; the actual mutation is `revokeRole`) lived in one doc and propagated into a new doc via copy-paste from the issue body that itself carried the typo. Resolve every cross-doc symbol by grepping the schema or source on the same edit.

**The same rule applies to CLI subcommands.** A doc that references `supabase db remote sql` looks plausible to any reviewer who has not run the command — only running `supabase db remote --help` reveals that the subcommand does not exist. Before committing any CLI invocation in a doc or playbook, verify the subcommand exists by running the tool's help (`<cmd> --help` or `<cmd> <subcmd> --help`). Do not rely on a sibling doc or issue body that uses the same invocation: the prior doc may have contained the same typo.

## Markdown anchor links over bare-text references

Cross-doc references should be Markdown anchor links — `[\`docs/foo.md\` § "Section title"](foo.md#section-title)` — not bare text — `See \`docs/foo.md\` § "Section title"`. The anchor link is rot-loud: a heading rename breaks the anchor and a CI link-checker catches it. A bare-text reference is rot-silent: the heading can drift arbitrarily and nothing complains until a reader tries to follow it.

GitHub's slug rules for the anchor portion: lowercase the heading, replace spaces with hyphens, drop backticks, **keep underscores as-is**. So `## Bootstrap admin via \`SUPER_USER_EMAILS\`` slugs to `#bootstrap-admin-via-super_user_emails` (underscore preserved). Symbols other than `_` and `-` are dropped, not transliterated. When in doubt, render the doc on github.com once and copy the anchor from the heading's hover-link.

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

Run the CJK grep **before writing** any new UI string literals (JSX text, `placeholder`, `aria-label`, etc.), not after. Memory and convention assumptions about "matching existing UX text" are unreliable — grep is the canonical proof. A design-doc claim that "the text matches the rest of the app" was disproven by grep during the card-duplicate-overwrite implementation: the target component was the only file in `frontend/src/` carrying CJK content, requiring a full English rewrite. The grep takes under one second; discovering the violation in review wastes far more.
