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

**The same rule extends to plan-document references** — `(per plan §6)`, `(see plan §10)`, `(plan §F-1)` and similar parentheticals in source comments or committed docs. Plan documents under `.claude/plans/` are scratch artifacts: their section numbering changes as the plan is revised, and the plan file itself is often deleted or moved once the work lands. A source comment that anchors its justification to "plan §6" reads naturally to a future contributor while pointing at a heading that no longer exists. Phrase the justification as a standing rule and link to the relevant `.claude/rules/` or `docs/` section instead — those have stable URLs and are subject to the rest of this language policy. Reference: a comment in `frontend/src/components/nav/global-rail.tsx` originally read "light up Users (per plan §6 / decision F-1)" and was rewritten as "light up Users when /admin/<unknown> falls through, so the rail always points at a valid admin destination" — the standing-statement form survives any future plan-numbering change.

## Verify cross-doc symbols against the source of truth, never against a sibling doc

When a doc references a GraphQL mutation name, a Go function name, an env-var name, or any other code-level identifier, the source of truth is `schema/schema.graphql`, the Go source file, or the canonical config — **never** a sibling doc. A typo in `docs/A.md` that gets copied verbatim into `docs/B.md` looks self-consistent (both docs agree) and survives review by anyone who checks A and B against each other instead of against the schema. The failure mode that exposed this rule: a stale admin role-mutation reference lived in one doc and propagated into a new doc via copy-paste from an issue body that itself carried the typo. Resolve every cross-doc symbol by grepping the schema or source on the same edit.

**The same rule applies to CLI subcommands.** A doc that references `supabase db remote sql` looks plausible to any reviewer who has not run the command — only running `supabase db remote --help` reveals that the subcommand does not exist. Before committing any CLI invocation in a doc or playbook, verify the subcommand exists by running the tool's help (`<cmd> --help` or `<cmd> <subcmd> --help`). Do not rely on a sibling doc or issue body that uses the same invocation: the prior doc may have contained the same typo.

**The same rule applies to PR body descriptions of code behavior.** A PR body that describes what a function returns in a particular branch must be verified against the production return statement, not inferred from adjacent test names. A worked example: a PR body described `TestAuthorizeCardgroupOrBadInput_NonOwner_ReturnsUnauthenticated` as returning a `ValidationError` — the test name says "Unauthenticated" but the description used "ValidationError" from a different branch. Grepping the production file for `return` near the non-owner branch would have caught the mismatch in seconds. Treat error-type claims in PR bodies the same as identifier references: grep the source to verify before committing the body.

**The same rule applies after a refactor deletes a Go type or function.** A doc that uses the deleted symbol as a worked example looks self-consistent until a reader greps the source and finds nothing. After landing a refactor that removes a type or function, run `grep -rn <DeletedSymbol> docs/` across the whole doc tree — not just the docs adjacent to the changed code — because worked examples hide in seemingly unrelated rule files that teach an adjacent pattern. If the example is load-bearing for the doc's thesis, rewrite it with a current symbol; if the reference was incidental, drop it entirely rather than leaving a pointer to a symbol that no longer exists. The same grep applies to **test files**, not just `docs/`. A refactor that renames or removes a function leaves stale identifiers behind in test comments (worked example: issue #181 commit 4829b40 refreshed three `authorizeCardgroup` references in `swipe_performance_test.go` that no longer named any production symbol). Test-file comments rot as silently as `docs/` references and need the same post-refactor grep — `grep -rn <DeletedSymbol> backend/` covers both production and test trees.

**The same grep must cover `.claude/rules/` and the L3 chapters they link to.** Rule files teach patterns by citing real production symbols as worked examples. When a symbol used as a worked example in a rule file is renamed or deleted, the rule's worked example silently rots — a future reader who follows it will grep for the symbol and find nothing. The canonical post-refactor grep is therefore:

```bash
grep -rn <DeletedSymbol> backend/ frontend/src/ frontend/__tests__/ docs/ .claude/rules/
```

A `uuidV7` → `domain.NewID` rename left a stale identifier in [`.claude/rules/subagent-dispatch.md`](subagent-dispatch.md) under the "Symbol moves must be atomic" section; the grep over `docs/` and `backend/` alone missed it because rules live under `.claude/rules/`. The frontend test suite lives in two trees — co-located tests under `frontend/src/**` and broad page tests plus shared utilities/fixtures under `frontend/__tests__/` — so a grep scoped to `frontend/src/` alone misses a deleted test util cited by a worked example under `frontend/__tests__/utils/`; include both paths.

**The same post-refactor grep applies after a TypeScript/TSX file is renamed.** When you rename a component file (e.g. PascalCase → kebab-case: `AdminUsersClient.tsx` → `admin-users-client.tsx`), run:

```bash
grep -rn 'OldFileName' frontend/ docs/ .claude/rules/
```

Two distinct hit categories need updating:

1. **Import statements in test files that used the old path.** TypeScript does not enforce these automatically on case-insensitive file systems (macOS HFS+) — local tests pass but CI on Linux (case-sensitive ext4) fails with `tsc --noEmit` exit 2 and uncollected vitest suites.
2. **Doc prose that cited the old path as a worked example.** These rot silently — readers grep the referenced path and find nothing.

Both categories were missed in the initial kebab-case rename of `AdminUsersClient.tsx` / `AdminRolesClient.tsx` and an admin user edit client on this branch, requiring a two-commit follow-up (`fix(tests): update admin client imports to kebab-case paths` and `docs: update stale admin client paths after kebab-case rename`). Always run `npx tsc --noEmit` immediately after a file rename — case-insensitive local file systems silently accept the stale imports.

Component-identifier references (e.g. `<AdminUsersClient />` JSX, `AdminUsersClient` as a TypeScript identifier) stay as-is — React convention keeps component names PascalCase even when the filename is kebab-case. Only file-path references convert.

**Mechanical rewrites preserve broken `§` anchors silently.** A regex/sed replacement that swaps `docs/foo.md` → `bar/CLAUDE.md` across N files preserves the `§ "X"` suffix on every site. If `## X` was only ever a heading in `docs/foo.md` and the new target is a different document, the anchor falls through silently — GitHub resolves unknown fragments to the page top without a 404. The verification gate is to walk each rewritten line and confirm the heading the `§` suffix names exists in the new target, before committing. A worked example from this repository: nine sites preserved `frontend/CLAUDE.md § "X"` anchors during the `docs/frontend.md` → `frontend/CLAUDE.md` migration. The target was a 49-line orientation hub with three `##` headings; none of the nine `X` strings matched. Each site was repointed at the actual `docs/frontend/<chapter>.md` file with the verified GitHub slug.

**A docblock claim about a module's *consumers* must be verified by import-grep on the module, not API-grep on the wrapped API.** "Who uses this module" is answered only by grepping the module's path or name in import statements; grepping the API the module wraps answers a different question — "who calls the wrapped API" — and the two result sets diverge exactly when a consumer bypasses the module and talks to the wrapped API directly. A docblock that names its consumers off the wrong grep over-claims. A worked example from this repository: a shared test mock util's scope docblock claimed "the middleware consumes this factory" on the basis of `grep -rln "auth.getUser" frontend/src/` (callers of the Supabase API), but the middleware mocks `@supabase/ssr` directly and never imports the util — the authoritative check was `grep -rln "mock-supabase" frontend/` (importers of the module). Verify consumer claims with the module-import grep:

```bash
grep -rln '<module-path-or-name>' frontend/ backend/   # who imports this module
```

## Markdown anchor links over bare-text references

Cross-doc references should be Markdown anchor links — `[\`docs/foo.md\` § "Section title"](foo.md#section-title)` — not bare text — `See \`docs/foo.md\` § "Section title"`. The anchor link is rot-loud: a heading rename breaks the anchor and a CI link-checker catches it. A bare-text reference is rot-silent: the heading can drift arbitrarily and nothing complains until a reader tries to follow it.

GitHub's slug rules for the anchor portion: lowercase the heading, replace spaces with hyphens, drop backticks, **keep underscores as-is**. So `## Bootstrap admin via \`SUPER_USER_EMAILS\`` slugs to `#bootstrap-admin-via-super_user_emails` (underscore preserved). Symbols other than `_` and `-` are dropped, not transliterated. When in doubt, render the doc on github.com once and copy the anchor from the heading's hover-link.

### Anchor links to index files silently drop the anchor portion

When a large doc is split into chapter files and an index file lists them, an anchor link of the form `index.md#some-section-heading` whose heading now lives in `chapters/some-section.md` silently resolves to the index top — GitHub drops the unmatched anchor rather than returning a 404. The link looks correct in source but lands at the wrong place with no warning. Always link to the **chapter file** (`chapters/some-section.md`), not the index with an anchor. See [`docs/doc-organization.md` § "Anchor links must point at the chapter file, not the index"](../../docs/doc-organization.md#anchor-links-must-point-at-the-chapter-file-not-the-index).

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
