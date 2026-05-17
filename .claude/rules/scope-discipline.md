# Scope discipline

> Applies to: every request. Cross-cutting workflow rule about what *not* to do before answering.

## Don't grep the codebase to answer external questions

Questions about external platforms — GCP IAM, GitHub Environments semantics, Vercel build hooks, Supabase auth internals, npm tarball layout — cannot be answered by reading this repo. The repo only knows how *we* configure the platform; it does not know what the platform does. Examples that wasted time in past sessions:

- "Does GCP progressively remove `setIamPolicy` from `roles/editor`?" → answered by GCP IAM release notes, not by `iam.tf`.
- "What does GitHub Environments require for protection rules?" → answered by GitHub docs, not by `.github/workflows/*.yml`.

For external questions, fetch the official docs (`WebFetch`) or run a probing CLI (`gcloud … describe`, `gh api …`) and cite the source. If you can't verify, say `unverified` explicitly rather than reasoning from memory.

## Don't split files unless they exceed the documented threshold

When asked to apply a doc-tier or file-size convention (see `docs/doc-organization.md` and the L1/L2/L3 tier rules in `CLAUDE.md`), only split files that **actually exceed** the documented soft cap. Splitting files preemptively because "they're getting close" creates churn, breaks anchor links across the repo, and inverts the cost/benefit of the convention. Measure first (`wc -l`), split second.

## Verify claims about external platforms before stating them as fact

Statements about GCP, Vercel, Supabase, GitHub, or any other vendor's behavior must be backed by a primary source on the same edit:

- Vendor docs page (`WebFetch` the URL).
- Vendor CLI output (`gcloud`, `vercel`, `supabase`, `gh api`).
- Repo-local config that *is* the source of truth (e.g. `iam.tf` for our GCP IAM, but only for our config — never for vendor behavior).

A confident-sounding wrong answer about platform behavior costs more downstream than a hedged "I haven't verified this" answer up front.

The same principle applies to **library behavior disputes** between agents or between agent and reviewer. When two
agents disagree on what a library does (e.g. whether `usePathname()` returns a decoded or percent-encoded string),
resolve the disagreement by consulting a primary source — read the installed package source, run the code, or fetch
the official changelog — on the same edit. Do not settle the dispute by majority vote or by deferring to whichever
side sounds more confident. Two concrete cases from this codebase:

- `usePathname` encoded vs decoded — settled by reading installed `next@16.2.4` source: `new URL(canonicalUrl, ...).pathname` preserves reserved characters as `%XX`. The "decoded" reading of the docs was misleading; the primary source was the installed code, not a summary.
- `pointer-events-none` vestigial vs load-bearing — settled by reasoning about `sticky`'s overflow-overlay semantics: the dead-zone cost is invisible (no interactive element sits under the safe-area padding zone), while removing `pointer-events-none` blocks scroll gestures near the bar in overflow viewports. The primary source was the CSS spec behavior, confirmed by the layout constraints.

## Plan-document claims about the codebase's structure must be toolchain-verified

When a plan asserts a structural property of *our own* code — "no import cycle", "no transitive dependency on package X", "this package compiles standalone", "this generic type erases to a single instantiation" — the proof must come from the toolchain (`go build`, `go list`, `go vet`, `tsc --noEmit`), not from a manual reading of the import statements or type signatures listed in the plan. The plan author is reasoning about a snapshot of intent; the toolchain reasons about the actual current source tree, including legacy state the plan summary may have abstracted away.

A worked example: a plan asserted that adding a `gqlerr → usecase` import would not create a cycle, on the basis that the new shared types in `usecase` had no `gqlerr` import. The legacy `usecase/*.go` production files already imported `gqlerr` (a fact the plan summary did not surface), and the proposed direct import would have closed a cycle the compiler rejects. A 5-second smoke build (`echo 'package main; import _ "backend/internal/gqlerr"; func main() {}' | go build -`, scoped to the proposed layout) catches this before any other infrastructure is wired. Run the toolchain check before committing to a directory layout; do not settle the question by re-reading the plan.

A second worked example: before writing a `go-arch-lint` archfile that encodes 21 component dependency rules, run `go list -f '{{join .Imports "\n"}}' ./internal/usecase/...` (and analogous invocations for every component) to enumerate the actual imports. Any mismatch between the plan's stated `mayDependOn` list and the real import set produces either a violation at first run (too restrictive) or a silent gap in enforcement (too permissive). The pre-flight `go list` survey takes under a minute for the whole tree and eliminates an entire class of "rule doesn't match reality" iteration rounds.
