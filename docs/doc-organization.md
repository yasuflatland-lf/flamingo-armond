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

## Parallel agent safety for doc splits

Splitting multiple rule files in parallel (one agent per file) is safe when:

1. Each agent **owns one source rule file end-to-end** and does not touch another agent's source file.
2. All agents are **forbidden from editing each other's source rule**.
3. Edits to **shared sink files** (source comments referencing moved paths, sibling rule cross-references) happen at different anchors — verify this before dispatching.

Doubly-edited files are safe when the two agents edit non-overlapping anchors. Conflicts arise only when two agents touch the same anchor in the same file.
