# `as string` cast on regex captures under `noUncheckedIndexedAccess`

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

The frontend tsconfig enables `noUncheckedIndexedAccess`, which widens `RegExpExecArray[number]` to `string | undefined`. For a capture group the regex makes mandatory (i.e. the regex cannot match without producing that capture), the soundest pattern is `const id = match[1] as string;` paired with a comment naming the invariant the cast relies on:

```ts
// frontend/src/components/nav/header-create-action.ts
const editMatch = CARDGROUP_EDIT_RE.exec(pathname);
if (editMatch) {
  // editMatch[1] is always defined when the regex matched (capture group 1 is required)
  const rawId = safeDecodePathSegment(editMatch[1] as string);
  if (rawId === null) return null;
  return cardWithGroup(rawId);
}

const learnMatch = LEARN_RE.exec(pathname);
if (learnMatch) {
  // learnMatch[1] is always defined when the regex matched (capture group 1 is required)
  const rawId = safeDecodePathSegment(learnMatch[1] as string);
  if (rawId === null) return null;
  return cardWithGroupFromLearn(rawId);
}
```

Avoid `String(match[1] ?? "")` — that turns `undefined` into the literal string `"undefined"`, which silently corrupts downstream URLs. Avoid the bare non-null assertion `match[1]!` because it offers no docstring anchor for the invariant: a future contributor reading `match[1]!` cannot tell whether the assertion is sound or a leftover from a refactor.

**Why:** `noUncheckedIndexedAccess` is a project-wide flag that future contributors may not be aware of. Without the comment, the `as string` cast looks superfluous and is a candidate for "cleanup" by anyone reading the code in isolation. The comment names the invariant (capture group N is required by this regex) so the cast survives review.

**How to apply:** for every regex-capture access where the capture is required by the regex, use `as string` with a one-line comment naming the required capture group. The comment is load-bearing — do not delete it during refactoring. The same pattern applies to other `noUncheckedIndexedAccess`-affected accesses (e.g. `Object.keys(o)[0]`); the rule is "explain the invariant, not just satisfy the compiler." Reference: `frontend/src/components/nav/header-create-action.ts` (`editMatch[1] as string`, `learnMatch[1] as string`).
