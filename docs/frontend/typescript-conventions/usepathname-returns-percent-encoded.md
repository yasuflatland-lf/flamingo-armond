# `usePathname()` returns percent-encoded path segments — decode before interpolating into hrefs

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

`usePathname()` (Next.js App Router) returns the **percent-encoded** pathname, not a decoded string.
`URL.pathname` preserves most "reserved" characters (`&`, `=`, `+`, `@`, `:` etc.) as `%XX` escape sequences.
The official docs describe the return value as "the decoded URL pathname of the current URL", but this refers to
decoded *unsafe* characters (spaces, non-ASCII) — reserved characters such as `&` stay encoded as `%26`.
Verified against the installed `next@16.2.4` source: the value is produced by `new URL(canonicalUrl, ...).pathname`,
which does not apply `decodeURIComponent` to the whole segment.

## Why it matters for nav components

A component on `/learn/my%20deck` that naively does:

```ts
const pathname = usePathname();          // "/learn/my%20deck"
const id = pathname.split("/learn/")[1]; // "my%20deck"
// DON'T: passing already-encoded string to encodeURIComponent double-encodes it
const href = `/cards/new?cardgroup=${encodeURIComponent(id)}`;
// Result: /cards/new?cardgroup=my%2520deck  ← double-encoded!
```

produces a double-encoded URL (`%2520` instead of `%20`).

## Correct pattern

1. **Extract** the raw segment from the pathname (regex capture or `split`).
2. **Decode** it with [`safeDecodePathSegment`](../../../frontend/src/lib/safe-decode-path-segment.ts) — returns `null` on malformed `%XX` sequences instead of throwing.
3. **Store** the result as the raw id.
4. **Encode** once at href construction time with `encodeURIComponent`.

```ts
const learnMatch = pathname.match(/^\/learn\/([^/]+)$/);
// safeDecodePathSegment returns null on URIError — callers must handle null.
const rawId = learnMatch ? safeDecodePathSegment(learnMatch[1] as string) : null;

// href construction — encode exactly once:
const href = rawId
  ? `/cards/new?cardgroup=${encodeURIComponent(rawId)}&return=/learn/${encodeURIComponent(rawId)}`
  : null;
```

## Anti-pattern: passing the captured segment straight into `encodeURIComponent`

Passing the captured segment (already percent-encoded by `usePathname`) directly into `encodeURIComponent`
double-encodes any `%XX` sequence in the id:

```ts
// WRONG — if pathname is /learn/foo%26bar, match[1] is "foo%26bar",
// and encodeURIComponent("foo%26bar") → "foo%2526bar"
const encodedId = encodeURIComponent(match[1] as string);
```

The bug is invisible for plain alphanumeric UUIDs but surfaces immediately for any id containing `&`, `+`,
spaces, or other characters that `URL.pathname` keeps encoded.

## `null` return is load-bearing — do not replace with `!`

`safeDecodePathSegment` returns `null` when `decodeURIComponent` throws a `URIError` (malformed `%XX`).
Layout-level nav components (rendered by `app/layout.tsx`) must handle the `null` case by skipping the
link rather than crashing — a `URIError` at that level escapes every route segment's `error.tsx` and
crashes the whole shell. See
[`.claude/rules/frontend-rsc-error-handling.md` § "Header (root layout) MUST degrade on failure, never throw"](../../../.claude/rules/frontend-rsc-error-handling.md#header-root-layout-must-degrade-on-failure-never-throw).

## Consumers

- `frontend/src/components/nav/logo-drawer.tsx` — extracts the `/learn/[id]` segment from `usePathname()`,
  decodes via `safeDecodePathSegment`, then re-encodes at href construction.
- `frontend/src/components/nav/fab-action.ts` — same pattern for both the `/learn/[id]` and
  `/cardgroups/[id]/edit` branches.
