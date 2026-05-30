# CI bash `for f in $(ls glob)` silently passes on zero matches under `set -e` — use `shopt -s nullglob` + an array

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

GitHub Actions runs every `run:` block as `bash --noprofile --norc -eo pipefail`, so `set -e` and `pipefail` are active. A loop written as `for f in $(ls -1 dir/*.up.sql | sort)` looks safe, but when the glob matches zero files the failure is swallowed: a command substitution in the word-list of a `for` compound command is one of the documented positions where a non-zero exit does NOT trigger `set -e`, and `pipefail` does not help because the failing `ls` lives inside `$()`. The `for` body never executes and the step exits 0. In a CI pipeline that then publishes generated output, this ships an empty artifact while the job stays green.

```bash
# BROKEN - zero-match glob silently passes under `set -e`
for f in $(ls -1 backend/internal/database/migrations/*.up.sql | sort); do
  apply "$f"
done
```

Verified empirically: `bash -c 'set -eo pipefail; for f in $(ls /nope/*.x); do :; done; echo reached'` prints `reached` and exits 0. The non-zero `ls` never aborts the step.

**The fix is to expand the glob into an array with `shopt -s nullglob` (so a zero match becomes an empty array rather than the literal glob string), then guard on the array length before the loop.** Glob expansion is already lexically sorted, so the `| sort` pipe is unnecessary too.

```bash
# CORRECT - nullglob + array + explicit empty-check fails loudly
set -euo pipefail
shopt -s nullglob
files=(backend/internal/database/migrations/*.up.sql)
if [[ ${#files[@]} -eq 0 ]]; then
  echo "::error::no migration files found - aborting." >&2
  exit 1
fi
for f in "${files[@]}"; do
  apply "$f"
done
```

**How to apply:** Any CI loop over a glob must use `nullglob` + an array + an explicit `${#arr[@]} -eq 0` guard; never `for x in $(ls glob)`. This is the glob-iteration sibling of the `rc=$?`-under-`set -e` gotcha documented in [`ci-bash-rc-capture-under-set-e.md`](ci-bash-rc-capture-under-set-e.md). Real instance: the migration-apply step in `.github/workflows/er-chart.yml`.
