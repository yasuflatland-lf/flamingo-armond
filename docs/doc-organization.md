# Documentation organization

How to classify, split, and maintain the doc tiers defined in `CLAUDE.md`.

## Tier system

`CLAUDE.md` defines three tiers:

- **L1 `CLAUDE.md`** — top-level orientation, ≤ 35 lines.
- **L2 `docs/`** — architecture and topic docs. Soft cap ~600 lines per file; split by topic when a section exceeds that.
- **L3 `.claude/rules/`** — cross-cutting rules auto-loaded into every Claude Code turn.

The L3 directory is the most expensive tier per-token. Use the 5-axis framework below to decide whether new content belongs there.

## 5-axis auto-load classification

A rule belongs in **auto-load** (`.claude/rules/`) when 4 or more of these hold:

| Axis | Auto-load signal |
|---|---|
| A. Frequency | Applies to nearly every change in the relevant tree |
| B. Discoverability | Agent would not know to look for it |
| C. Violation cost | Silent regression; no CI catch |
| D. Token budget | Token cost is negligible relative to value |
| E. Orthogonality | Encodes a single invariant, not a grab-bag |

A rule belongs in **on-demand** (`docs/`) when 4 or more of:

| Axis | On-demand signal |
|---|---|
| A. Frequency | Only when touching a specific library or pattern |
| B. Discoverability | Filename or symbol is greppable |
| C. Violation cost | Typecheck, test, or CI catches it |
| D. Token budget | Would materially multiply per-turn cost |
| E. Orthogonality | A collection of unrelated patterns for one area |

### Tier 1 vs Tier 2 vs Tier 3

**Tier 1 — auto-load as-is**: universal invariants with a small token footprint (e.g. `language-policy.md`).

**Tier 2 — split**: a large rule file whose top section meets the auto-load criteria but whose depth content does not. Keep the short core in `.claude/rules/<file>.md` and move each subsection to a chapter file under `docs/<destination>/`. Replace the cut content with a pointer list:

```md
## Detailed cases (on-demand)

- [Section title](../../docs/<destination>/<chapter>.md)
```

When a topic-area heading already exists (e.g. `## Server-side design`) and you are inlining a handful of stubs underneath it, do **not** add a sibling `## Server-side design (on-demand)` for the pointer list — the two headings collide on the topic and produce two anchor slugs (`server-side-design` and `server-side-design-on-demand`) that fragment incoming links. Use a single `##` heading for the topic and put the pointer list under a `### Further reading (on-demand)` subsection inside it. A doc-cleanup pass that inlined stubs first produced the duplicate-heading shape before being normalized to the single-heading + subsection form now used in `pagination.md`.

**Tier 3 — move entirely**: files where no section meets the auto-load criteria. Move all content to `docs/` and replace the `.claude/rules/` file with a 4-line pointer stub.

## Splitting a large rule file into chapters

### Anchor links must point at the chapter file, not the index

When a large rule file is split into chapters and an index file is created to list them, cross-doc anchor links must point at the **chapter file**, not the index. An anchor like `index.md#some-section-heading` whose heading now lives in `chapters/some-section.md` will be silently dropped by GitHub — the link resolves the index file but lands at the page top rather than the target heading, with no 404 or warning. Always link to `chapters/some-section.md` (optionally with `#heading-in-that-file`), not to the index with an anchor.

This extends the rule in [`.claude/rules/language-policy.md` § "Markdown anchor links over bare-text references"](../.claude/rules/language-policy.md#markdown-anchor-links-over-bare-text-references).

### Describe code locations by structure, not line number

Doc references to source lines (`see lines 22–28`, `the throw at line 81`) rot every time the file is edited. The reader following the doc lands in the wrong place silently — GitHub will not warn that the cited line range no longer reflects the cited content. Phrase locations structurally:

- Name the function: "the `hasAuthError` helper".
- Cite the guard or branch: "after the `if (json.data != null)` guard".
- Reference the surrounding section: "near the top of the `if (json.errors)` block".

Each of these survives line shifts. Line numbers belong in commit messages or diff comments, not in committed docs.

#### CI gate

`scripts/check-doc-line-citations.sh` enforces this rule. It runs in `.github/workflows/docs.yml`, in the same job as `check-claude-md-hierarchy.sh`. It scans every `*.md` under `docs/` and `.claude/rules/` for `<file>.(go|ts|tsx|sql|graphql):<N>` and prints the offending file, line number and line before exiting non-zero. Run it locally before pushing:

```bash
bash scripts/check-doc-line-citations.sh
```

Two carve-outs, both structural rather than per-file allowlists:

- **Fenced regions are skipped.** Quoted compiler, linter and stack-trace output legitimately carries line numbers, and it is always pasted inside a fenced block. Keeping such output inside a fence is therefore the sanctioned way to record it; a citation the author writes in prose has no fence and fails.
- **`docs/superpowers/plans/**` is excluded.** Plan documents are dated point-in-time snapshots — the same category as issue bodies and PR descriptions — so rewriting their citations is churn rather than maintenance.

### Inline `§ "above"` / `§ "below"` references rot on split

When a single rule file is split into chapters, prose references like "§ section above" or "see the rule below" become meaningless — the referenced section now lives in a sibling file, not the same document. Two compliant postures:

1. **Rewrite**: replace `§ "Foo bar" above` with a relative link `[§ "Foo bar"](./foo-bar.md)`.
2. **Accept and document**: note in the chapter's opening comment that some cross-references still use prose form from before the split.

Leaving them unchanged silently rots the navigation — a reader following the prose is told to look "above" but the section no longer exists in the same file.

### Path-swap sweeps: sanity-check adjacent punctuation

When a path-update sweep uses regex substitution (e.g. replacing `.claude/rules/foo.md` with `docs/bar.md` across many files), verify that adjacent punctuation was not accidentally changed. A comma in `"per `.claude/rules/foo.md`, this spy is..."` can become a colon if the regex matched more than intended. After any mechanical path-swap, run a `git diff --word-diff` or `grep` against the edited lines to confirm only the path portion changed. The symptom is subtle enough that it survives code review and only surfaces in a dedicated simplifier pass.

### Scope the path-update sweep to the exact filenames you moved

A grep for bare filenames (e.g. `grep -rn "frontend-rsc-error-handling"`) catches references that a `grep "\.claude/rules/frontend-rsc-error-handling\.md"` misses — source comments and test-file prose often reference doc sections by bare filename without the full path. Run both forms:

```bash
grep -rn "\.claude/rules/<file>\.md" --include="*.md" --include="*.ts" --include="*.tsx" --include="*.go"
grep -rn "<bare-filename-stem>" --include="*.md" --include="*.ts" --include="*.tsx" --include="*.go"
```

Missing the bare-filename sweep leaves stale section references in test-file prose (e.g. `"per frontend-rsc-error-handling.md § \"Redact ...\""`) that point at sections which no longer exist in the parent file because they moved to a chapter.

### Grep the directory you are purging, not only its outside callers

When deleting a stub from `docs/<area>/`, run the cross-reference grep **inside** `docs/<area>/` as well as outside it. A sweep that only checks "incoming from elsewhere" misses sibling chapter files in the same directory that link to the deleted stub — they are co-located, so a grep scoped to "every other doc" silently excludes them. A doc-cleanup pass declared a stub safe to delete after the outside-only grep returned empty, and two sibling chapters in the same directory were still linking to it; the broken anchors only surfaced in a separate verification pass. Run `grep -rn "<deleted-filename-stem>" docs/<area>/` in addition to the repo-wide sweep before removing.

### Anchor regexes drop variant filename forms

A regex like `global-header\.tsx` matches the production file but not `global-header.test.tsx` — the `.test.` infix breaks the anchor. When sweeping for stale references to a deleted source file, use a **substring** grep on the filename stem (`grep -rn "global-header"`), not a `\.tsx`-anchored regex. The same gotcha applies to `*.spec.ts` and any file family that decorates the stem with an infix before the extension. A code-drift sweep originally anchored on the production extension and missed the stale reference in the matching test file; the substring form caught it on a follow-up pass.

## Prose enumerations override plan headings during execution

A plan document under `.claude/plans/` that says "Phase 4a: inline 13 stubs" but whose prose body lists fifteen filenames is internally inconsistent — the heading and the prose disagree on the count. When executing such a phase, the **prose list** is authoritative because it names the specific files; the heading count is a summary written from memory and rots first. Before acting on a phase, reconcile the count by `wc -l` on the prose bullet list or by greppping the named files, and proceed against that count. Treating the heading number as authoritative produces silent under- or over-execution. Worked example: a doc-cleanup phase headed "13 stubs" listed fifteen files in the prose; the inline pass that trusted the prose count completed correctly, while a count-by-heading would have left two stubs behind.

## Parallel agent safety for doc splits

Splitting multiple rule files in parallel (one agent per file) is safe when:

1. Each agent **owns one source rule file end-to-end** and does not touch another agent's source file.
2. All agents are **forbidden from editing each other's source rule**.
3. Edits to **shared sink files** (source comments referencing moved paths, sibling rule cross-references) happen at different anchors — verify this before dispatching.

Doubly-edited files are safe when the two agents edit non-overlapping anchors. Conflicts arise only when two agents touch the same anchor in the same file.

## Hierarchical CLAUDE.md scheme

In addition to the L1/L2/L3 tier docs, area-specific `CLAUDE.md` files at the top of each major directory carry the area's topic-doc index. The four area files are:

- [`backend/CLAUDE.md`](../backend/CLAUDE.md) — Go / Echo backend orientation and chapter index.
- [`frontend/CLAUDE.md`](../frontend/CLAUDE.md) — TypeScript / GraphQL frontend orientation and chapter index.
- [`schema/CLAUDE.md`](../schema/CLAUDE.md) — shared GraphQL schema orientation.
- [`docs/CLAUDE.md`](CLAUDE.md) — doc-tier conventions and layout pointer.

Claude Code auto-loads ancestor `CLAUDE.md` files based on the current working directory, so backend-only work no longer pulls the frontend chapter list into context (and vice versa).

### What belongs in an area `CLAUDE.md`

- Layout pointers (`cmd/`, `internal/`, `src/`, …).
- Area-specific quickstart commands (`go run ./cmd/server`, `pnpm --filter frontend dev`).
- A Markdown-hyperlinked index of the area's L2 chapter docs (`docs/<area>/...md`).
- A short "Cross-cutting rules already in context" footer naming the `.claude/rules/` files auto-loaded alongside it, so the file does not duplicate their content.

**What does NOT belong**: cross-cutting rules (language policy, error wrapping, RSC error handling, …). Those stay in `.claude/rules/` and auto-load orthogonally to cwd.

### Soft caps

Same spirit as L1's 35-line cap. Actual sizes in the current tree: `backend/CLAUDE.md` ~30 lines, `frontend/CLAUDE.md` ~50 lines, `schema/CLAUDE.md` ~20 lines, `docs/CLAUDE.md` ~27 lines. When a section grows, push the detail down to a `docs/<area>/<topic>.md` chapter and leave the area file a pointer.

**Topic-doc references must be Markdown hyperlinks, not backtick-only text.** The CI check described below greps `](...)` patterns; a bare-text reference escapes verification and the next file-rename will silently rot the index.

## CI verification: `scripts/check-claude-md-hierarchy.sh`

The script runs in `.github/workflows/docs.yml` and enforces four invariants:

1. **Presence** — each of the four area `CLAUDE.md` files exists.
2. **Absence** — `docs/frontend.md` does **not** exist (it was migrated to `frontend/CLAUDE.md`; this gate catches accidental reintroduction).
3. **Coverage** — every direct child of `docs/frontend/*.md` is listed in `frontend/CLAUDE.md`'s topic-docs section.
4. **Link validity** — every Markdown-link target inside the four area `CLAUDE.md` files resolves to a real file.

The check is scoped to area `CLAUDE.md` files. It deliberately does **not** validate links inside L2/L3 chapter docs — that is a follow-up candidate for a fuller link checker (e.g. `lychee`, `markdown-link-check`).

### Bash portability gotchas fixed in the script

The initial implementation hit two macOS-incompatible idioms under `set -euo pipefail`. Both are documented here so future maintainers do not reintroduce them:

- **`find -printf` is GNU-only.** `find` on BSD/macOS does not support `-printf`. The portable substitute is `find ... -exec basename {} \;`, which works on both GNU and BSD `find`.
- **`grep | sed | while ... exit 1` is silently swallowed.** A subshell `exit 1` terminates only the subshell, not the parent script. Additionally, when the leading `grep` in a pipeline returns exit code 1 for a no-match (a legitimate outcome when a file contains zero Markdown links), `set -euo pipefail` aborts the entire script before the `while` body runs. The fix is process substitution with an error counter:

  ```bash
  # Correct — process substitution avoids the subshell + pipefail trap
  while IFS= read -r link; do
    # ... validate $link ...
    link_errors=$((link_errors + 1))
  done < <(grep -oE '\]\(([^)]+)\)' "$f" 2>/dev/null | sed -E '...' || true)
  if [[ $link_errors -ne 0 ]]; then exit 1; fi

  # Incorrect — exit 1 in the while body terminates only the subshell
  grep ... | sed ... | while ...; do
    exit 1   # silently swallowed
  done
  ```

  The `|| true` after the `sed` invocation prevents `pipefail` from aborting when `grep` finds no matches; the `while` loop then correctly iterates over zero lines.
