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
